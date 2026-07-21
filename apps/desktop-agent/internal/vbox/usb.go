package vbox

import (
	"context"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// extensionPackName is the exact display name VBoxManage prints for the Oracle
// Extension Pack in `list extpacks`. USB 2.0/3.0 passthrough needs it.
const extensionPackName = "Oracle VirtualBox Extension Pack"

// usbAttachedFieldLabels are the labels VBoxManage emits for each device inside
// the human-readable "Currently Attached USB Devices:" block. They bound the
// section: any other label-bearing line starts a different section and ends the
// attached-USB list (see parseAttachedUsbUUIDs).
var usbAttachedFieldLabels = map[string]bool{
	"uuid":          true,
	"vendorid":      true,
	"productid":     true,
	"revision":      true,
	"manufacturer":  true,
	"product":       true,
	"serialnumber":  true,
	"address":       true,
	"port":          true,
	"current state": true,
	"backend":       true,
}

// VmUsb lists the host's USB devices and the two prerequisites the UI surfaces:
// whether the Oracle Extension Pack is installed and whether the VM has a USB
// controller enabled. Each device is marked attachedHere when this VM has
// currently captured it.
//
// The host device list (`list usbhost`) is host-global, so it goes through the
// global concurrency cap (s.exec) rather than the per-VM gate. The controller
// flags and the attached-device UUIDs come from this VM's showvminfo. Extension
// pack presence is host-global. The host-device list, attachment read, and
// extension-pack read are all best-effort: a failure degrades that one piece
// (empty list / not-attached / not-installed) rather than failing the whole read.
func (s *service) VmUsb(ctx context.Context, id string) (models.VmUsbResponse, error) {
	if !IsValidVmID(id) {
		return models.VmUsbResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.VmUsbResponse{}, err
	}

	// USB controller flags are read from machine-readable showvminfo. A failure
	// here fails the whole read because it is the VM's own required state.
	info, err := s.readShowVmInfo(ctx, path, id, "reading USB controller configuration")
	if err != nil {
		return models.VmUsbResponse{}, err
	}
	controllerEnabled := parseUsbControllerEnabled(info)

	devices := make([]models.UsbDevice, 0)
	if res, lerr := s.exec(ctx, path, listUsbHostArgs(), 10*time.Second); lerr == nil && res.ExitCode == 0 {
		devices = parseUsbHostDevices(res.StandardOutput)
	}

	// `list usbhost` does not say which VM captured a device, so the set of
	// devices attached to THIS VM comes from the human-readable showvminfo
	// "Currently Attached USB Devices:" section, matched against the host list
	// by UUID. Best-effort: a failure leaves everything not-attached.
	if human, herr := s.readShowVmInfoHuman(ctx, path, id, "reading attached USB devices"); herr == nil {
		attached := parseAttachedUsbUUIDs(human)
		for i := range devices {
			if _, ok := attached[strings.ToLower(devices[i].UUID)]; ok {
				devices[i].AttachedHere = true
			}
		}
	}

	extInstalled := false
	if res, eerr := s.exec(ctx, path, listExtpacksArgs(), 10*time.Second); eerr == nil && res.ExitCode == 0 {
		extInstalled = parseExtensionPackInstalled(res.StandardOutput)
	}

	return models.VmUsbResponse{
		Devices:                devices,
		ExtensionPackInstalled: extInstalled,
		USBControllerEnabled:   controllerEnabled,
	}, nil
}

// AttachUsb captures a host USB device into a running VM
// (`controlvm <id> usbattach <uuid>`).
func (s *service) AttachUsb(ctx context.Context, id, deviceUUID string) (models.UsbOperationResponse, error) {
	return s.usbAction(ctx, id, deviceUUID, true)
}

// DetachUsb releases a captured USB device from a running VM
// (`controlvm <id> usbdetach <uuid>`).
func (s *service) DetachUsb(ctx context.Context, id, deviceUUID string) (models.UsbOperationResponse, error) {
	return s.usbAction(ctx, id, deviceUUID, false)
}

// usbAction is the shared attach/detach path. Both are live operations, so the
// VM must be running; a stopped VM is rejected with a clear ValidationError.
// Known passthrough failures (missing USB controller, missing Extension Pack,
// unavailable host USB proxy) are mapped to actionable messages.
func (s *service) usbAction(ctx context.Context, id, deviceUUID string, attach bool) (models.UsbOperationResponse, error) {
	action := "vm.usb.detach"
	verb := "detaching USB device"
	if attach {
		action = "vm.usb.attach"
		verb = "attaching USB device"
	}

	if !IsValidVmID(id) {
		return models.UsbOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if !IsValidUsbDeviceID(deviceUUID) {
		return models.UsbOperationResponse{}, &ValidationError{Message: "Invalid USB device identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, action, false, "VirtualBox/VBoxManage not discovered.")
		return models.UsbOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state before USB change")
	if err != nil {
		return models.UsbOperationResponse{}, err
	}
	if !vmStateIsLive(parseVmState(info)) {
		vErr := &ValidationError{Message: "The VM must be running to attach or detach USB devices."}
		s.logOperation(ctx, id, action, false, vErr.Message)
		return models.UsbOperationResponse{}, vErr
	}

	args := usbDetachArgs(id, deviceUUID)
	if attach {
		args = usbAttachArgs(id, deviceUUID)
	}
	if err := s.runControlCommand(ctx, id, path, args, verb); err != nil {
		s.logOperation(ctx, id, action, false, controlFailureMessage(verb, err))
		return models.UsbOperationResponse{}, mapUsbControlError(err)
	}

	s.logOperation(ctx, id, action, true, "")
	message := "USB device detached from the VM."
	if attach {
		message = "USB device attached to the VM."
	}
	return models.UsbOperationResponse{Success: true, VMID: id, Message: message}, nil
}

// mapUsbControlError turns known usbattach/usbdetach stderr signatures into a
// clear, actionable ValidationError (HTTP 400). Anything else passes through as
// the original ExecutionError so the server maps it to a generic 502.
func mapUsbControlError(err error) error {
	execErr, ok := err.(*ExecutionError)
	if !ok {
		return err
	}
	stderr := strings.ToLower(execErr.StandardError)
	contains := func(needle string) bool {
		return strings.Contains(stderr, strings.ToLower(needle))
	}
	switch {
	case contains("usb proxy service") || contains("vboxusb") || contains("host usb"):
		return &ValidationError{
			Message: "The host USB service is unavailable. Reinstall VirtualBox so its USB driver is registered, then reconnect the device.",
		}
	case contains("extension pack") || contains("extpack") || contains("usb 2.0") || contains("usb 3.0"):
		return &ValidationError{
			Message: "Attaching this device needs the Oracle VirtualBox Extension Pack. Install it, then try again.",
		}
	case contains("no usb controller") || contains("usb controller"):
		return &ValidationError{
			Message: "This VM has no USB controller enabled. Power the VM off and enable USB in its settings first.",
		}
	default:
		return execErr
	}
}

// parseUsbHostDevices parses `VBoxManage list usbhost` into one entry per
// device. Devices are separated by blank lines and each begins with a "UUID:"
// line; the labels are read with afterLabel, mirroring parseHostInterfaceNames.
// The returned slice is never nil so it serializes as [] rather than null.
func parseUsbHostDevices(output string) []models.UsbDevice {
	devices := make([]models.UsbDevice, 0)
	var cur *models.UsbDevice
	flush := func() {
		if cur != nil && cur.UUID != "" {
			devices = append(devices, *cur)
		}
		cur = nil
	}

	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if v, ok := afterLabel(line, "UUID:"); ok {
			flush()
			cur = &models.UsbDevice{UUID: strings.TrimSpace(v)}
			continue
		}
		if cur == nil {
			continue
		}
		// ProductId must be checked before Product so the "Product:" label does
		// not shadow it (it does not, since neither is a prefix of the other, but
		// the ordering keeps the intent explicit).
		if v, ok := afterLabel(line, "VendorId:"); ok {
			cur.VendorID = firstField(v)
		} else if v, ok := afterLabel(line, "ProductId:"); ok {
			cur.ProductID = firstField(v)
		} else if v, ok := afterLabel(line, "Manufacturer:"); ok {
			cur.Manufacturer = strings.TrimSpace(v)
		} else if v, ok := afterLabel(line, "Product:"); ok {
			cur.Product = strings.TrimSpace(v)
		} else if v, ok := afterLabel(line, "Current State:"); ok {
			cur.State = strings.TrimSpace(v)
		}
	}
	flush()
	return devices
}

// parseAttachedUsbUUIDs returns the set of host USB device UUIDs (lowercased)
// that the VM currently has captured, parsed from the human-readable showvminfo
// "Currently Attached USB Devices:" section. `list usbhost` does not attribute a
// device to a VM, so this is how attachment is determined. The section is bounded
// by the known per-device field labels: the first line carrying a different
// label (e.g. the next section header) ends the list, so a UUID that only appears
// in a later section ("Available remote USB devices:") is never counted.
func parseAttachedUsbUUIDs(human string) map[string]struct{} {
	attached := map[string]struct{}{}
	inSection := false

	for _, raw := range strings.Split(human, "\n") {
		line := strings.TrimSpace(raw)
		if !inSection {
			if rest, ok := afterLabel(line, "Currently Attached USB Devices:"); ok {
				if strings.Contains(strings.ToLower(rest), "none") {
					return attached
				}
				inSection = true
			}
			continue
		}

		if line == "" {
			continue
		}
		if v, ok := afterLabel(line, "UUID:"); ok {
			u := strings.ToLower(strings.TrimSpace(v))
			if uuidPattern.MatchString(u) {
				attached[u] = struct{}{}
			}
			continue
		}
		label, _, hasColon := strings.Cut(line, ":")
		if hasColon && !usbAttachedFieldLabels[strings.ToLower(strings.TrimSpace(label))] {
			// A label that is not part of a device block starts a new section.
			break
		}
	}
	return attached
}

// parseUsbControllerEnabled reports whether the VM has any USB controller
// enabled, from machine-readable showvminfo. VirtualBox exposes three controller
// types — usb (OHCI/1.1), ehci (2.0), xhci (3.0) — and any one being "on" means
// the VM can capture USB devices.
func parseUsbControllerEnabled(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		switch key {
		case "usb", "ehci", "xhci":
			if strings.EqualFold(value, "on") {
				return true
			}
		}
	}
	return false
}

// parseExtensionPackInstalled reports whether `VBoxManage list extpacks` lists
// the Oracle Extension Pack, which is required for USB 2.0/3.0 passthrough.
func parseExtensionPackInstalled(output string) bool {
	return strings.Contains(strings.ToLower(output), strings.ToLower(extensionPackName))
}

// firstField returns the first whitespace-separated token of s (e.g. "0x0781"
// from "0x0781 (0781)"), or "" when s is blank.
func firstField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// IsValidUsbDeviceID reports whether id is a canonical UUID, the shape
// VBoxManage prints for host USB devices. usbattach/usbdetach also accept a
// device address or vendorid:productid, but TabVM only ever passes a UUID from
// the host list, so validation matches that.
func IsValidUsbDeviceID(id string) bool {
	return uuidPattern.MatchString(id)
}

func listUsbHostArgs() []string {
	return []string{"list", "usbhost"}
}

func listExtpacksArgs() []string {
	return []string{"list", "extpacks"}
}

func usbAttachArgs(id, deviceUUID string) []string {
	return []string{"controlvm", id, "usbattach", deviceUUID}
}

func usbDetachArgs(id, deviceUUID string) []string {
	return []string{"controlvm", id, "usbdetach", deviceUUID}
}
