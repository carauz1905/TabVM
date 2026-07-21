package vbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// resizableFormats are the disk formats TabVM can grow with `modifymedium
// --resize`. VirtualBox reliably resizes VDI and VHD; VMDK resize is limited to
// specific variants and often refused, so it is excluded to avoid failed or
// half-applied operations.
var resizableFormats = map[string]bool{"VDI": true, "VHD": true}

// mediumDetails is the subset of `showmediuminfo` output needed to decide
// whether a disk can be resized.
type mediumDetails struct {
	format      string
	variant     string
	capacityMB  int64
	allocatedMB int64
	hasChildren bool
}

// VmStorage lists a VM's attached hard disks with the metadata needed to resize
// them: format, current capacity, and whether a resize is possible. A disk is
// resizable only when the VM is powered off, the format is VDI/VHD, it is not a
// fixed-size image, and it has no snapshots (child media).
func (s *service) VmStorage(ctx context.Context, id string) (models.VmStorageResponse, error) {
	if !IsValidVmID(id) {
		return models.VmStorageResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmStorageResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM storage")
	if err != nil {
		return models.VmStorageResponse{}, err
	}
	editable := !vmStateIsLive(parseVmState(info))

	disks := make([]models.DiskInfo, 0)
	for _, att := range parseDiskAttachments(info) {
		if att.uuid == "" {
			continue
		}
		med, medErr := s.runner.RunContext(ctx, path, showMediumInfoArgs(att.uuid), 10*time.Second)
		if medErr != nil || med.ExitCode != 0 {
			continue
		}
		det := parseMediumDetails(med.StandardOutput)
		resizable, reason := diskResizable(det, editable)
		disks = append(disks, models.DiskInfo{
			UUID:        att.uuid,
			Name:        att.name,
			Format:      det.format,
			CapacityMB:  det.capacityMB,
			AllocatedMB: det.allocatedMB,
			Resizable:   resizable,
			Reason:      reason,
		})
	}

	// The optical (DVD) drive is read from the human-readable showvminfo because
	// the machine-readable form does not label an attachment as dvd vs hdd. It is
	// best-effort: a failure to read it leaves Optical.Present false rather than
	// failing the whole storage read.
	var optical models.OpticalDrive
	if human, herr := s.readShowVmInfoHuman(ctx, path, id, "reading VM optical drive"); herr == nil {
		od := parseOpticalDrive(human)
		optical = models.OpticalDrive{
			Present:    od.present,
			Medium:     od.medium,
			Name:       baseName(od.medium),
			Controller: od.controller,
			Port:       od.port,
			Device:     od.device,
		}
	}

	return models.VmStorageResponse{ID: id, Disks: disks, Optical: optical, Editable: editable}, nil
}

// ResizeDisk grows a virtual disk to sizeMB. VirtualBox can only enlarge a disk
// (never shrink), and only while it is not in use, so a live VM is refused. The
// guest filesystem is not touched — this enlarges the container only; the guest
// partition must be expanded separately inside the OS.
func (s *service) ResizeDisk(ctx context.Context, id, uuid string, sizeMB int64) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if !IsValidVmID(uuid) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid disk identifier."}
	}
	if sizeMB < 1 {
		return models.VmOperationResponse{}, &ValidationError{Message: "New disk size must be a positive number of megabytes."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "disk.resize", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state before disk resize")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before resizing a disk."}
	}

	med, medErr := s.runner.RunContext(ctx, path, showMediumInfoArgs(uuid), 10*time.Second)
	if medErr != nil || med.ExitCode != 0 {
		return models.VmOperationResponse{}, &ExecutionError{
			ExitCode:      med.ExitCode,
			StandardError: med.StandardError,
			Message:       "VBoxManage failed while reading the disk before resize",
		}
	}
	det := parseMediumDetails(med.StandardOutput)
	if resizable, reason := diskResizable(det, true); !resizable {
		return models.VmOperationResponse{}, &ValidationError{Message: reason}
	}
	if sizeMB <= det.capacityMB {
		return models.VmOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Disks can only grow. Enter a size larger than the current %d MB.", det.capacityMB)}
	}

	// Growing a large disk rewrites metadata and can take a while.
	if err := s.runControlCommandTimeout(ctx, path, resizeDiskArgs(uuid, sizeMB), "resizing disk", 10*time.Minute); err != nil {
		s.logOperation(ctx, id, "disk.resize", false, "VBoxManage modifymedium resize failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "disk.resize", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Disk resized to %d MB. Expand the partition inside the guest to use the new space.", sizeMB),
	}, nil
}

// minDiskMB and maxDiskMB bound a newly created disk: at least 1 GB (a smaller
// disk is rarely useful) and at most 2 TB (a conservative ceiling).
const (
	minDiskMB = 1024
	maxDiskMB = 2 * 1024 * 1024
)

var (
	errNoSataController = errors.New("no SATA controller")
	errControllerFull   = errors.New("SATA controller is full")
)

// storageController is one of a VM's disk controllers, parsed from showvminfo,
// with the ports already occupied by a device.
type storageController struct {
	name         string
	ctlType      string
	portCount    int
	maxPortCount int
	used         map[int]bool
}

// AddDisk creates a new dynamically-allocated VDI of sizeMB and attaches it to a
// free SATA port on the VM, growing the controller's port count if needed. The
// VM must be powered off. The new disk is placed alongside the VM's config file.
func (s *service) AddDisk(ctx context.Context, id string, sizeMB int64) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if sizeMB < minDiskMB {
		return models.VmOperationResponse{}, &ValidationError{Message: fmt.Sprintf("New disk must be at least %d MB (1 GB).", minDiskMB)}
	}
	if sizeMB > maxDiskMB {
		return models.VmOperationResponse{}, &ValidationError{Message: "New disk cannot exceed 2 TB."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "disk.add", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM storage before adding a disk")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before adding a disk."}
	}

	ctlName, port, bumpTo, pickErr := pickSataTarget(parseStorageControllers(info))
	if pickErr != nil {
		if errors.Is(pickErr, errControllerFull) {
			return models.VmOperationResponse{}, &ValidationError{Message: "The SATA controller has no free ports left."}
		}
		return models.VmOperationResponse{}, &ValidationError{Message: "This VM has no SATA controller to attach a disk to."}
	}

	cfg := parseCfgFile(info)
	if cfg == "" {
		return models.VmOperationResponse{}, &ExecutionError{Message: "Could not determine the VM folder for the new disk."}
	}
	dir := filepath.Dir(cfg)
	base := strings.TrimSuffix(filepath.Base(cfg), filepath.Ext(cfg))
	diskPath, pathErr := nextDiskPath(dir, base)
	if pathErr != nil {
		return models.VmOperationResponse{}, &ExecutionError{Message: "Could not choose a filename for the new disk."}
	}

	if err := s.runControlCommandTimeout(ctx, path, createDiskMBArgs(diskPath, sizeMB), "creating disk", 5*time.Minute); err != nil {
		s.logOperation(ctx, id, "disk.add", false, "VBoxManage createmedium failed.")
		return models.VmOperationResponse{}, err
	}
	if bumpTo > 0 {
		if err := s.runControlCommand(ctx, path, setPortCountArgs(id, ctlName, bumpTo), "growing the storage controller"); err != nil {
			s.logOperation(ctx, id, "disk.add", false, "VBoxManage storagectl portcount failed.")
			return models.VmOperationResponse{}, err
		}
	}
	if err := s.runControlCommand(ctx, path, storageAttachAtArgs(id, ctlName, port, diskPath), "attaching disk"); err != nil {
		s.logOperation(ctx, id, "disk.add", false, "VBoxManage storageattach failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "disk.add", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Added a %d MB disk on %s port %d.", sizeMB, ctlName, port),
	}, nil
}

// pickSataTarget finds a SATA controller and a port to attach a new disk to. It
// returns a free port within the current port count (bumpTo 0), or the next port
// with bumpTo set to the port count the controller must grow to.
func pickSataTarget(cs []storageController) (name string, port, bumpTo int, err error) {
	sawSata := false
	for _, c := range cs {
		if !strings.Contains(strings.ToLower(c.ctlType), "ahci") {
			continue
		}
		sawSata = true
		for p := 0; p < c.portCount; p++ {
			if !c.used[p] {
				return c.name, p, 0, nil
			}
		}
		if c.portCount < c.maxPortCount {
			return c.name, c.portCount, c.portCount + 1, nil
		}
	}
	if sawSata {
		return "", 0, 0, errControllerFull
	}
	return "", 0, 0, errNoSataController
}

// parseStorageControllers extracts each controller's name, type, port count, and
// the ports already occupied, from machine-readable showvminfo output.
func parseStorageControllers(output string) []storageController {
	byIndex := map[int]*storageController{}
	order := []int{}
	get := func(i int) *storageController {
		if _, ok := byIndex[i]; !ok {
			byIndex[i] = &storageController{used: map[int]bool{}}
			order = append(order, i)
		}
		return byIndex[i]
	}

	for _, line := range strings.Split(output, "\n") {
		if key, value, ok := splitMachineReadable(line); ok {
			if idx, ok := indexSuffix(key, "storagecontrollername"); ok {
				get(idx).name = value
			} else if idx, ok := indexSuffix(key, "storagecontrollertype"); ok {
				get(idx).ctlType = value
			} else if idx, ok := indexSuffix(key, "storagecontrollermaxportcount"); ok {
				get(idx).maxPortCount, _ = strconv.Atoi(value)
			} else if idx, ok := indexSuffix(key, "storagecontrollerportcount"); ok {
				get(idx).portCount, _ = strconv.Atoi(value)
			}
		}
	}

	// Attachment slots ("<name>-<port>-<device>") keep their case, so use the
	// raw-key splitter and match each slot to its controller by name.
	byName := map[string]*storageController{}
	for _, c := range byIndex {
		byName[c.name] = c
	}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadableRawKey(line)
		if !ok {
			continue
		}
		m := attachmentKeyRe.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		c, found := byName[m[1]]
		if !found {
			continue
		}
		if value == "" || strings.EqualFold(value, "none") {
			continue
		}
		if p, err := strconv.Atoi(m[2]); err == nil {
			c.used[p] = true
		}
	}

	controllers := make([]storageController, 0, len(order))
	for _, i := range order {
		controllers = append(controllers, *byIndex[i])
	}
	return controllers
}

// parseCfgFile returns the VM's settings-file path, unescaping the doubled
// backslashes VBoxManage emits in machine-readable output.
func parseCfgFile(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if key, value, ok := splitMachineReadableRawKey(line); ok && key == "CfgFile" {
			return strings.ReplaceAll(value, `\\`, `\`)
		}
	}
	return ""
}

// nextDiskPath returns the first non-existing "<base>_<n>.vdi" path in dir.
func nextDiskPath(dir, base string) (string, error) {
	for n := 1; n <= 1000; n++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d.vdi", base, n))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", errors.New("no free disk filename")
}

// indexSuffix reports whether key is prefix followed by a non-negative integer
// index (e.g. "storagecontrollername0") and returns that index.
func indexSuffix(key, prefix string) (int, bool) {
	if !strings.HasPrefix(key, prefix) {
		return 0, false
	}
	suffix := key[len(prefix):]
	if suffix == "" {
		return 0, false
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func createDiskMBArgs(diskPath string, sizeMB int64) []string {
	return []string{"createmedium", "disk", "--filename", diskPath, "--size", strconv.FormatInt(sizeMB, 10), "--format", "VDI"}
}

func setPortCountArgs(id, ctlName string, count int) []string {
	return []string{"storagectl", id, "--name", ctlName, "--portcount", strconv.Itoa(count)}
}

func storageAttachAtArgs(id, ctlName string, port int, diskPath string) []string {
	return []string{"storageattach", id, "--storagectl", ctlName, "--port", strconv.Itoa(port), "--device", "0", "--type", "hdd", "--medium", diskPath}
}

// DetachDisk removes a disk from the VM. With deleteFile false the .vdi is left
// on disk (fully reversible — it can be re-attached later); with deleteFile true
// the image is deleted after detaching, which is irreversible. The VM must be
// powered off, and a disk with snapshots cannot be deleted.
func (s *service) DetachDisk(ctx context.Context, id, uuid string, deleteFile bool) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if !IsValidVmID(uuid) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid disk identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "disk.detach", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM storage before detaching a disk")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	if vmStateIsLive(parseVmState(info)) {
		return models.VmOperationResponse{}, &ValidationError{Message: "The VM is running. Power it off before detaching a disk."}
	}

	ctl, port, device, found := findDiskAttachment(info, uuid)
	if !found {
		return models.VmOperationResponse{}, &ValidationError{Message: "That disk is not attached to this VM."}
	}

	if deleteFile {
		med, medErr := s.runner.RunContext(ctx, path, showMediumInfoArgs(uuid), 10*time.Second)
		if medErr == nil && med.ExitCode == 0 && parseMediumDetails(med.StandardOutput).hasChildren {
			return models.VmOperationResponse{}, &ValidationError{Message: "This disk has snapshots. Delete them before deleting the disk."}
		}
	}

	if err := s.runControlCommand(ctx, path, detachDiskArgs(id, ctl, port, device), "detaching disk"); err != nil {
		s.logOperation(ctx, id, "disk.detach", false, "VBoxManage storageattach detach failed.")
		return models.VmOperationResponse{}, err
	}

	if !deleteFile {
		s.logOperation(ctx, id, "disk.detach", true, "")
		return models.VmOperationResponse{Success: true, VMID: id, Message: "Disk detached from the VM. Its file was kept and can be re-attached later."}, nil
	}

	// Deleting the image can rewrite metadata; allow a little longer.
	if err := s.runControlCommandTimeout(ctx, path, closeMediumDeleteArgs(uuid), "deleting disk image", 5*time.Minute); err != nil {
		s.logOperation(ctx, id, "disk.detach", false, "VBoxManage closemedium --delete failed.")
		return models.VmOperationResponse{}, err
	}
	s.logOperation(ctx, id, "disk.detach", true, "deleted")
	return models.VmOperationResponse{Success: true, VMID: id, Message: "Disk detached and its file permanently deleted."}, nil
}

// findDiskAttachment locates the controller, port, and device a medium UUID is
// attached at, from machine-readable showvminfo output.
func findDiskAttachment(output, uuid string) (ctl string, port, device int, found bool) {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadableRawKey(line)
		if !ok {
			continue
		}
		m := imageUUIDKeyRe.FindStringSubmatch(key)
		if m == nil || !strings.EqualFold(value, uuid) {
			continue
		}
		p, perr := strconv.Atoi(m[2])
		d, derr := strconv.Atoi(m[3])
		if perr != nil || derr != nil {
			continue
		}
		return m[1], p, d, true
	}
	return "", 0, 0, false
}

func detachDiskArgs(id, ctl string, port, device int) []string {
	return []string{"storageattach", id, "--storagectl", ctl, "--port", strconv.Itoa(port), "--device", strconv.Itoa(device), "--type", "hdd", "--medium", "none"}
}

func closeMediumDeleteArgs(uuid string) []string {
	return []string{"closemedium", "disk", uuid, "--delete"}
}

// diskResizable applies the resize policy to a medium's details and returns a
// human-readable reason when it is not resizable.
func diskResizable(det mediumDetails, editable bool) (bool, string) {
	if !resizableFormats[strings.ToUpper(det.format)] {
		return false, fmt.Sprintf("Only VDI and VHD disks can be resized (this one is %s).", det.format)
	}
	if strings.Contains(strings.ToLower(det.variant), "fixed") {
		return false, "Fixed-size disks cannot be resized."
	}
	if det.hasChildren {
		return false, "This disk has snapshots. Delete them before resizing."
	}
	if !editable {
		return false, "Power off the VM to resize its disks."
	}
	return true, ""
}

// parseMediumDetails extracts the format, variant, capacity, allocation, and
// snapshot (child) state from `showmediuminfo` output.
func parseMediumDetails(output string) mediumDetails {
	var det mediumDetails
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := afterLabel(line, "Storage format:"); ok {
			det.format = strings.TrimSpace(v)
		} else if v, ok := afterLabel(line, "Format variant:"); ok {
			det.variant = strings.TrimSpace(v)
		} else if v, ok := afterLabel(line, "Capacity:"); ok {
			det.capacityMB = parseSizeWithUnit(v) >> 20
		} else if v, ok := afterLabel(line, "Size on disk:"); ok {
			det.allocatedMB = parseSizeWithUnit(v) >> 20
		} else if _, ok := afterLabel(line, "Child UUIDs:"); ok {
			det.hasChildren = true
		}
	}
	return det
}

func resizeDiskArgs(uuid string, sizeMB int64) []string {
	return []string{"modifymedium", "disk", uuid, "--resize", fmt.Sprintf("%d", sizeMB)}
}
