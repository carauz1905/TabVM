package vbox

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tabvm/desktop-agent/internal/models"
)

// The optical (DVD) drive is discovered from the HUMAN-READABLE `showvminfo <id>`
// output rather than the machine-readable form, because machine-readable does not
// label an attachment as dvd vs hdd. The human-readable storage block, verified
// against VBoxManage 7.2.12, looks like:
//
//	Storage Controllers:
//	#0: 'SATA', Type: IntelAhci, Instance: 0, Ports: 2 (max 30), Bootable
//	  Port 0, Unit 0: UUID: 846b821b-...
//	    Location: "C:\...\disk.vdi"
//	  Port 1, Unit 0: UUID: 107cb914-...
//	    Location: "C:\...\alpine.iso"
//	  Port 1, Unit 0: Empty            <- an empty removable (DVD) drive
//
// Disambiguation (the human-readable form has no explicit type column): an
// attachment is the optical drive when its medium is "Empty" or its medium is
// NOT a hard-disk image (identified by extension). This recognizes any optical
// medium — an ISO, a host-DVD passthrough, a .viso — not just ".iso", while
// excluding the hard disks. A hard disk is never reported as "Empty" (an
// unattached hard-disk port is omitted entirely). Only the first optical drive
// is returned, since a VM normally has exactly one.
var (
	controllerHeaderRe = regexp.MustCompile(`^#\d+:\s+'([^']*)'`)
	attachmentLineRe   = regexp.MustCompile(`^Port (\d+), Unit (\d+):\s*(.*)$`)
)

// opticalDrive is a VM's optical (DVD) drive location and current medium.
type opticalDrive struct {
	controller string
	port       int
	device     int
	medium     string // absolute ISO path, or "" when the drive is empty
	present    bool
}

// parseOpticalDrive scans human-readable showvminfo output and returns the VM's
// optical (DVD) drive. present is false when the VM has no optical drive.
func parseOpticalDrive(human string) opticalDrive {
	lines := strings.Split(human, "\n")
	currentCtl := ""
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if m := controllerHeaderRe.FindStringSubmatch(trimmed); m != nil {
			currentCtl = m[1]
			continue
		}
		m := attachmentLineRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		port, _ := strconv.Atoi(m[1])
		device, _ := strconv.Atoi(m[2])
		rest := strings.TrimSpace(m[3])

		if strings.EqualFold(rest, "Empty") {
			// An empty removable drive: the optical drive with no disc inserted.
			return opticalDrive{controller: currentCtl, port: port, device: device, present: true}
		}
		if !strings.HasPrefix(rest, "UUID:") {
			continue
		}
		// A medium is attached; its path is on the following Location: line. It is
		// the optical drive unless the medium is a hard-disk image.
		medium := locationAfter(lines, i)
		if medium != "" && !isHardDiskImage(medium) {
			return opticalDrive{controller: currentCtl, port: port, device: device, medium: medium, present: true}
		}
	}
	return opticalDrive{present: false}
}

// locationAfter returns the quoted path from the first `Location:` line following
// index i, stopping at the next attachment or controller header so it never picks
// up a neighbouring attachment's location.
func locationAfter(lines []string, i int) string {
	for j := i + 1; j < len(lines); j++ {
		next := strings.TrimSpace(lines[j])
		if strings.HasPrefix(next, "Location:") {
			val := strings.TrimSpace(strings.TrimPrefix(next, "Location:"))
			return strings.Trim(val, `"`)
		}
		if attachmentLineRe.MatchString(next) || controllerHeaderRe.MatchString(next) {
			return ""
		}
	}
	return ""
}

// isHardDiskImage reports whether p names a VirtualBox hard-disk image (by
// extension). Anything else attached to a removable drive — an ISO, a host-DVD
// passthrough, a .viso — is optical media, not a disk.
func isHardDiskImage(p string) bool {
	p = strings.ToLower(strings.TrimSpace(p))
	for _, ext := range []string{".vdi", ".vmdk", ".vhd", ".vhdx", ".hdd", ".qed", ".qcow", ".qcow2", ".raw"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// MountDvd inserts an ISO into the VM's optical (DVD) drive, resolving the drive's
// controller/port/device from the human-readable showvminfo. If the VM has no
// optical drive, a new dvddrive is attached to a free port on an existing
// controller (preferring SATA, matching the create flow). VirtualBox hot-swaps
// optical media, so this works whether the VM is running or stopped.
func (s *service) MountDvd(ctx context.Context, id, isoPath string) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if err := validateIsoPath(isoPath); err != nil {
		return models.VmOperationResponse{}, err
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.dvd.mount", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	ctl, port, device, bumpTo, err := s.resolveDvdTarget(ctx, path, id)
	if err != nil {
		s.logOperation(ctx, id, "vm.dvd.mount", false, "Could not resolve an optical drive target.")
		return models.VmOperationResponse{}, err
	}

	// Grow the controller first when a brand-new drive needs a port beyond the
	// current port count.
	if bumpTo > 0 {
		if err := s.runControlCommand(ctx, path, setPortCountArgs(id, ctl, bumpTo), "growing the storage controller"); err != nil {
			s.logOperation(ctx, id, "vm.dvd.mount", false, "VBoxManage storagectl portcount failed.")
			return models.VmOperationResponse{}, err
		}
	}
	if err := s.runControlCommand(ctx, path, storageAttachDvdMediumArgs(id, ctl, port, device, isoPath), "mounting ISO"); err != nil {
		s.logOperation(ctx, id, "vm.dvd.mount", false, "VBoxManage storageattach dvddrive failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "vm.dvd.mount", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Mounted %s into the DVD drive.", baseName(isoPath)),
	}, nil
}

// EjectDvd removes the medium from the VM's optical drive, leaving the drive in
// place (`--medium emptydrive`). Live-capable like MountDvd. Ejecting an
// already-empty drive is an idempotent success; a VM with no optical drive is
// rejected with a clear message.
func (s *service) EjectDvd(ctx context.Context, id string) (models.VmOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.VmOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "vm.dvd.eject", false, "VirtualBox/VBoxManage not discovered.")
		return models.VmOperationResponse{}, err
	}

	human, err := s.readShowVmInfoHuman(ctx, path, id, "reading VM optical drive before eject")
	if err != nil {
		return models.VmOperationResponse{}, err
	}
	od := parseOpticalDrive(human)
	if !od.present {
		return models.VmOperationResponse{}, &ValidationError{Message: "This VM has no optical drive to eject."}
	}
	if od.medium == "" {
		s.logOperation(ctx, id, "vm.dvd.eject", true, "already empty")
		return models.VmOperationResponse{Success: true, VMID: id, Message: "The DVD drive is already empty."}, nil
	}

	if err := s.runControlCommand(ctx, path, storageAttachDvdMediumArgs(id, od.controller, od.port, od.device, "emptydrive"), "ejecting ISO"); err != nil {
		s.logOperation(ctx, id, "vm.dvd.eject", false, "VBoxManage storageattach emptydrive failed.")
		return models.VmOperationResponse{}, err
	}

	s.logOperation(ctx, id, "vm.dvd.eject", true, "")
	return models.VmOperationResponse{
		Success: true,
		VMID:    id,
		Message: "Ejected the DVD medium; the drive is now empty.",
	}, nil
}

// resolveDvdTarget returns the controller, port, and device of the VM's optical
// drive. When the VM has an optical drive already, bumpTo is 0 and its exact
// location is returned. When it has none, a free port on an existing controller
// is chosen for a new dvddrive; bumpTo is non-zero when the controller must grow
// its port count first.
func (s *service) resolveDvdTarget(ctx context.Context, path, id string) (ctl string, port, device, bumpTo int, err error) {
	if human, herr := s.readShowVmInfoHuman(ctx, path, id, "reading VM optical drive"); herr == nil {
		if od := parseOpticalDrive(human); od.present {
			return od.controller, od.port, od.device, 0, nil
		}
	}

	// No optical drive: pick a free slot on an existing controller for a new one.
	info, ierr := s.readShowVmInfo(ctx, path, id, "reading VM storage for DVD mount")
	if ierr != nil {
		return "", 0, 0, 0, ierr
	}
	name, p, bump, ok := pickDvdTarget(parseStorageControllers(info))
	if !ok {
		return "", 0, 0, 0, &ValidationError{Message: "This VM has no optical drive and no free controller port to add one."}
	}
	return name, p, 0, bump, nil
}

// pickDvdTarget finds a controller and port to attach a new optical drive to,
// preferring a SATA/AHCI controller (matching the create flow) and falling back
// to any other controller. bumpTo is the port count a SATA controller must grow
// to when all its current ports are used; it is 0 for a free existing port.
func pickDvdTarget(cs []storageController) (name string, port, bumpTo int, ok bool) {
	if n, p, b, found := freeControllerPort(cs, true); found {
		return n, p, b, true
	}
	if n, p, b, found := freeControllerPort(cs, false); found {
		return n, p, b, true
	}
	return "", 0, 0, false
}

// freeControllerPort returns the first controller with a free port at device 0.
// When sataOnly is true only SATA/AHCI controllers are considered, and such a
// controller may grow its port count (bumpTo) when full.
func freeControllerPort(cs []storageController, sataOnly bool) (name string, port, bumpTo int, ok bool) {
	for _, c := range cs {
		isSata := strings.Contains(strings.ToLower(c.ctlType), "ahci")
		if sataOnly && !isSata {
			continue
		}
		for p := 0; p < c.portCount; p++ {
			if !c.used[p] {
				return c.name, p, 0, true
			}
		}
		if isSata && c.portCount < c.maxPortCount {
			return c.name, c.portCount, c.portCount + 1, true
		}
	}
	return "", 0, 0, false
}

// storageAttachDvdMediumArgs builds a storageattach command targeting a VM's
// optical drive. medium is an absolute ISO path to insert, or "emptydrive" to
// eject the current medium while keeping the drive.
func storageAttachDvdMediumArgs(id, ctl string, port, device int, medium string) []string {
	return []string{
		"storageattach", id,
		"--storagectl", ctl,
		"--port", strconv.Itoa(port),
		"--device", strconv.Itoa(device),
		"--type", "dvddrive",
		"--medium", medium,
	}
}
