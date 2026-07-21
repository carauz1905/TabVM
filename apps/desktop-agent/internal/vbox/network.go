package vbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// networkModes are the attachment modes TabVM can switch between. VirtualBox
// supports more (intnet, natnetwork, generic), but these three cover the lab
// cases students use: NAT for outbound internet, bridged for a real LAN IP, and
// host-only for an isolated host↔guest network.
var networkModes = map[string]bool{
	"nat":      true,
	"bridged":  true,
	"hostonly": true,
}

// NetworkOptions returns a VM's enabled NICs plus the host interfaces available
// for bridged and host-only attachment.
func (s *service) NetworkOptions(ctx context.Context, id string) (models.NetworkOptionsResponse, error) {
	if !IsValidVmID(id) {
		return models.NetworkOptionsResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		return models.NetworkOptionsResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading network adapters")
	if err != nil {
		return models.NetworkOptionsResponse{}, err
	}

	adapters := parseNetworkAdapters(info)

	// NAT port-forwarding rules cannot be read from --machinereadable: its
	// flat Forwarding(N)= index does not encode which NIC a rule belongs to.
	// The human-readable showvminfo labels each rule with its NIC number, so a
	// second, best-effort read attaches rules to the matching adapter. A failed
	// human read still returns the adapters, just without rules.
	if human, herr := s.readShowVmInfoHuman(ctx, path, id, "reading port forwarding rules"); herr == nil {
		rulesBySlot := parseForwardingRules(human)
		for i := range adapters {
			if rules := rulesBySlot[adapters[i].Slot]; len(rules) > 0 {
				adapters[i].Forwarding = rules
			}
		}
	}

	return models.NetworkOptionsResponse{
		Adapters:         adapters,
		BridgedAdapters:  s.listHostInterfaces(ctx, path, "bridgedifs"),
		HostOnlyAdapters: s.listHostInterfaces(ctx, path, "hostonlyifs"),
	}, nil
}

// ChangeNetworkMode switches a NIC's attachment mode. On a running VM the change
// is applied live with `controlvm nicN`; on a stopped VM it is written to the
// machine config with `modifyvm`. Bridged and host-only modes require a host
// interface to bind to.
func (s *service) ChangeNetworkMode(ctx context.Context, id string, slot int, mode, adapter string) (models.NetworkOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if slot < 1 || slot > 8 {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Network adapter slot must be between 1 and 8."}
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if !networkModes[mode] {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Network mode must be one of: nat, bridged, hostonly."}
	}
	adapter = strings.TrimSpace(adapter)
	if mode == "bridged" || mode == "hostonly" {
		if adapter == "" {
			return models.NetworkOperationResponse{}, &ValidationError{Message: "A host interface is required for bridged and host-only modes."}
		}
		if !isPlausibleHostInterface(adapter) {
			return models.NetworkOperationResponse{}, &ValidationError{Message: "Host interface name contains unsupported characters."}
		}
	} else {
		adapter = "" // NAT takes no interface
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "network.mode", false, "VirtualBox/VBoxManage not discovered.")
		return models.NetworkOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for network change")
	if err != nil {
		return models.NetworkOperationResponse{}, err
	}

	var args []string
	if vmStateIsLive(parseVmState(info)) {
		args = controlvmNicArgs(id, slot, mode, adapter)
	} else {
		args = modifyvmNicArgs(id, slot, mode, adapter)
	}

	if err := s.runControlCommand(ctx, id, path, args, "changing network mode"); err != nil {
		s.logOperation(ctx, id, "network.mode", false, "VBoxManage network mode change failed.")
		return models.NetworkOperationResponse{}, err
	}

	s.logOperation(ctx, id, "network.mode", true, "")
	message := fmt.Sprintf("Adapter %d switched to %s.", slot, networkModeLabel(mode))
	if adapter != "" {
		message = fmt.Sprintf("Adapter %d switched to %s (%s).", slot, networkModeLabel(mode), adapter)
	}
	return models.NetworkOperationResponse{Success: true, VMID: id, Message: message}, nil
}

// SetLinkState connects or disconnects a NIC's virtual network cable. On a
// running VM the change is applied live with `controlvm setlinkstateN`; on a
// stopped VM it is written to the machine config with `modifyvm --cableconnectedN`.
// A disconnected cable simulates unplugging the adapter without changing its
// attachment mode.
func (s *service) SetLinkState(ctx context.Context, id string, slot int, connected bool) (models.NetworkOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if slot < 1 || slot > 8 {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Network adapter slot must be between 1 and 8."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "network.link", false, "VirtualBox/VBoxManage not discovered.")
		return models.NetworkOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for network link change")
	if err != nil {
		return models.NetworkOperationResponse{}, err
	}

	var args []string
	if vmStateIsLive(parseVmState(info)) {
		args = setLinkStateArgs(id, slot, connected)
	} else {
		args = modifyLinkStateArgs(id, slot, connected)
	}

	if err := s.runControlCommand(ctx, id, path, args, "changing network link state"); err != nil {
		s.logOperation(ctx, id, "network.link", false, "VBoxManage network link change failed.")
		return models.NetworkOperationResponse{}, err
	}

	s.logOperation(ctx, id, "network.link", true, "")
	message := fmt.Sprintf("Adapter %d cable connected.", slot)
	if !connected {
		message = fmt.Sprintf("Adapter %d cable disconnected.", slot)
	}
	return models.NetworkOperationResponse{Success: true, VMID: id, Message: message}, nil
}

// listHostInterfaces returns the names of host interfaces for a `list`
// subcommand (bridgedifs or hostonlyifs). Best-effort: a failure yields an empty
// list so the UI still renders (the user just cannot pick that mode).
func (s *service) listHostInterfaces(ctx context.Context, path, kind string) []string {
	result, err := s.exec(ctx, path, []string{"list", kind}, 10*time.Second)
	if err != nil || result.ExitCode != 0 {
		return []string{}
	}
	return parseHostInterfaceNames(result.StandardOutput)
}

// parseNetworkAdapters extracts each enabled NIC's slot, mode, bound host
// interface, and MAC from machine-readable showvminfo output.
func parseNetworkAdapters(output string) []models.NetworkAdapter {
	modes := map[int]string{}
	macs := map[int]string{}
	bridged := map[int]string{}
	hostonly := map[int]string{}
	cable := map[int]string{}
	maxSlot := 0
	track := func(slot int) {
		if slot > maxSlot {
			maxSlot = slot
		}
	}

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		if slot, ok := slotSuffix(key, "nic"); ok {
			modes[slot] = strings.ToLower(value)
			track(slot)
		} else if slot, ok := slotSuffix(key, "macaddress"); ok {
			macs[slot] = normalizeMAC(value)
			track(slot)
		} else if slot, ok := slotSuffix(key, "bridgeadapter"); ok {
			bridged[slot] = value
			track(slot)
		} else if slot, ok := slotSuffix(key, "hostonlyadapter"); ok {
			hostonly[slot] = value
			track(slot)
		} else if slot, ok := slotSuffix(key, "cableconnected"); ok {
			cable[slot] = strings.ToLower(value)
			track(slot)
		}
	}

	adapters := make([]models.NetworkAdapter, 0, maxSlot)
	for slot := 1; slot <= maxSlot; slot++ {
		mode, ok := modes[slot]
		if !ok || mode == "" || mode == "none" {
			continue
		}
		// VBoxManage defaults cableconnected<N> to "on", so a missing key (empty
		// string) means the cable is plugged in; only an explicit "off" unplugs it.
		a := models.NetworkAdapter{Slot: slot, Mode: mode, MAC: macs[slot], CableConnected: cable[slot] != "off"}
		switch mode {
		case "bridged":
			a.Adapter = bridged[slot]
		case "hostonly":
			a.Adapter = hostonly[slot]
		}
		adapters = append(adapters, a)
	}
	return adapters
}

// parseHostInterfaceNames collects the "Name:" values from `VBoxManage list
// bridgedifs`/`hostonlyifs` output, one per interface block.
func parseHostInterfaceNames(output string) []string {
	names := make([]string, 0, 4)
	for _, line := range strings.Split(output, "\n") {
		if name, ok := afterLabel(strings.TrimSpace(line), "Name:"); ok {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// isPlausibleHostInterface is a defense-in-depth check on a host interface name
// before it is passed to VBoxManage. exec.Command bypasses the shell so this is
// not injectable, but rejecting control characters and a leading dash keeps
// unvalidated input from being read as a flag.
func isPlausibleHostInterface(name string) bool {
	if len(name) == 0 || len(name) > 256 || name[0] == '-' {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func networkModeLabel(mode string) string {
	switch mode {
	case "nat":
		return "NAT"
	case "bridged":
		return "bridged"
	case "hostonly":
		return "host-only"
	default:
		return mode
	}
}

// controlvmNicArgs changes a NIC live on a running VM.
func controlvmNicArgs(id string, slot int, mode, adapter string) []string {
	args := []string{"controlvm", id, "nic" + strconv.Itoa(slot), mode}
	if adapter != "" {
		args = append(args, adapter)
	}
	return args
}

// modifyvmNicArgs writes a NIC's mode into a stopped VM's config, binding the
// host interface for bridged/host-only in the same call.
func modifyvmNicArgs(id string, slot int, mode, adapter string) []string {
	n := strconv.Itoa(slot)
	args := []string{"modifyvm", id, "--nic" + n, mode}
	switch mode {
	case "bridged":
		args = append(args, "--bridgeadapter"+n, adapter)
	case "hostonly":
		args = append(args, "--hostonlyadapter"+n, adapter)
	}
	return args
}

// onOff maps a boolean link state to the VBoxManage "on"/"off" token.
func onOff(connected bool) string {
	if connected {
		return "on"
	}
	return "off"
}

// setLinkStateArgs connects or disconnects a NIC's virtual cable live on a
// running VM.
func setLinkStateArgs(id string, slot int, connected bool) []string {
	return []string{"controlvm", id, "setlinkstate" + strconv.Itoa(slot), onOff(connected)}
}

// modifyLinkStateArgs writes a NIC's cable state into a stopped VM's config.
func modifyLinkStateArgs(id string, slot int, connected bool) []string {
	return []string{"modifyvm", id, "--cableconnected" + strconv.Itoa(slot), onOff(connected)}
}
