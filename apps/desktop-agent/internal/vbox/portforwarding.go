package vbox

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

// nicRuleLineRe matches a single NAT port-forwarding rule line from the
// human-readable `showvminfo <id>` output, capturing the NIC number and the
// comma-separated field list, e.g.:
//
//	NIC 1 Rule(0):  name = ssh, protocol = tcp, host ip = , host port = 2222, guest ip = , guest port = 22
//
// The human-readable form is used deliberately: the --machinereadable output
// emits a flat Forwarding(N)= index that does not say which NIC a rule belongs
// to, so it cannot be attributed per NIC.
var nicRuleLineRe = regexp.MustCompile(`^NIC\s+(\d+)\s+Rule\(\d+\):\s*(.*)$`)

// parseForwardingRules extracts NAT port-forwarding rules from human-readable
// showvminfo output, keyed by NIC slot number.
func parseForwardingRules(output string) map[int][]models.PortForwardingRule {
	rules := map[int][]models.PortForwardingRule{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		m := nicRuleLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		slot, err := strconv.Atoi(m[1])
		if err != nil || slot <= 0 {
			continue
		}
		rules[slot] = append(rules[slot], parseForwardingFields(m[2]))
	}
	return rules
}

// parseForwardingFields parses the comma-separated "key = value" field list of a
// single rule line into a PortForwardingRule.
func parseForwardingFields(fields string) models.PortForwardingRule {
	rule := models.PortForwardingRule{}
	for _, part := range strings.Split(fields, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])
		switch key {
		case "name":
			rule.Name = value
		case "protocol":
			rule.Protocol = strings.ToLower(value)
		case "host ip":
			rule.HostIP = value
		case "host port":
			if n, err := strconv.Atoi(value); err == nil {
				rule.HostPort = n
			}
		case "guest ip":
			rule.GuestIP = value
		case "guest port":
			if n, err := strconv.Atoi(value); err == nil {
				rule.GuestPort = n
			}
		}
	}
	return rule
}

// AddPortForwarding adds a NAT port-forwarding rule to a NIC. On a running VM
// the rule is applied live with `controlvm natpfN`; on a stopped VM it is
// written to the config with `modifyvm --natpfN`. The target NIC must be in NAT
// mode, the rule name must be unique on that NIC, and the host port must not
// already be forwarded anywhere on this VM (host ports are global). An empty
// host IP defaults to 127.0.0.1 so a rule never binds all interfaces implicitly.
func (s *service) AddPortForwarding(ctx context.Context, id string, req models.PortForwardingRequest) (models.NetworkOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if req.Slot < 1 || req.Slot > 8 {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Network adapter slot must be between 1 and 8."}
	}
	proto := strings.ToLower(strings.TrimSpace(req.Protocol))
	if proto != "tcp" && proto != "udp" {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Protocol must be tcp or udp."}
	}
	name := strings.TrimSpace(req.Name)
	if !isPlausibleForwardingName(name) {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Rule name must be 1-64 characters, cannot contain a comma, and cannot start with a dash."}
	}
	if req.HostPort < 1 || req.HostPort > 65535 {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Host port must be between 1 and 65535."}
	}
	if req.GuestPort < 1 || req.GuestPort > 65535 {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Guest port must be between 1 and 65535."}
	}
	hostIP := strings.TrimSpace(req.HostIP)
	if hostIP == "" {
		// Default to loopback rather than binding all interfaces implicitly,
		// which would expose the guest port to the whole LAN.
		hostIP = "127.0.0.1"
	} else if net.ParseIP(hostIP) == nil {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Host IP is not a valid IP address."}
	}
	guestIP := strings.TrimSpace(req.GuestIP)
	if guestIP != "" && net.ParseIP(guestIP) == nil {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Guest IP is not a valid IP address."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "network.forwarding.add", false, "VirtualBox/VBoxManage not discovered.")
		return models.NetworkOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for port forwarding")
	if err != nil {
		return models.NetworkOperationResponse{}, err
	}

	// The target NIC must exist and be in NAT mode: port forwarding only applies
	// to NAT attachments.
	var target *models.NetworkAdapter
	for _, a := range parseNetworkAdapters(info) {
		if a.Slot == req.Slot {
			found := a
			target = &found
			break
		}
	}
	if target == nil {
		return models.NetworkOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Adapter %d is not enabled on this VM.", req.Slot)}
	}
	if target.Mode != "nat" {
		return models.NetworkOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Adapter %d must be in NAT mode to add a port-forwarding rule.", req.Slot)}
	}

	// Read the existing rules across all NICs to enforce uniqueness. The human
	// read is best-effort for display but authoritative for these guards, so a
	// failure to read it is treated as "no known rules" and the add proceeds
	// (VBoxManage still rejects a true duplicate).
	existing := map[int][]models.PortForwardingRule{}
	if human, herr := s.readShowVmInfoHuman(ctx, path, id, "reading existing port forwarding rules"); herr == nil {
		existing = parseForwardingRules(human)
	} else {
		// The read is best-effort, but silently skipping the uniqueness guards
		// would hide the bypass, so record that we are relying on VBoxManage alone.
		s.logOperation(ctx, id, "network.forwarding.add", false, "Could not read current rules; local uniqueness checks skipped (relying on VBoxManage).")
	}
	for _, r := range existing[req.Slot] {
		if strings.EqualFold(r.Name, name) {
			return models.NetworkOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Adapter %d already has a rule named %q.", req.Slot, name)}
		}
	}
	// Host ports are global on the host, not per-NIC: a host port+protocol pair
	// can be forwarded only once across the whole VM.
	for _, rs := range existing {
		for _, r := range rs {
			if r.HostPort == req.HostPort && strings.EqualFold(strings.TrimSpace(r.Protocol), proto) {
				return models.NetworkOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Host port %d/%s is already forwarded on this VM.", req.HostPort, proto)}
			}
		}
	}
	// Host ports are global to the whole host, not just this VM: if another
	// registered VM already forwards this host port+protocol, one of the two NAT
	// listeners would fail to bind at runtime. Best-effort — VMs whose info
	// cannot be read are skipped.
	if other := s.hostPortForwardedByOtherVM(ctx, path, id, proto, req.HostPort); other != "" {
		return models.NetworkOperationResponse{}, &ValidationError{Message: fmt.Sprintf("Host port %d/%s is already forwarded by VM %q.", req.HostPort, proto, other)}
	}

	rule := models.PortForwardingRule{
		Name:      name,
		Protocol:  proto,
		HostIP:    hostIP,
		HostPort:  req.HostPort,
		GuestIP:   guestIP,
		GuestPort: req.GuestPort,
	}
	live := vmStateIsLive(parseVmState(info))
	if err := s.runControlCommand(ctx, path, natpfAddArgs(id, req.Slot, rule, live), "adding port forwarding rule"); err != nil {
		s.logOperation(ctx, id, "network.forwarding.add", false, "VBoxManage port forwarding add failed.")
		return models.NetworkOperationResponse{}, err
	}

	s.logOperation(ctx, id, "network.forwarding.add", true, "")
	message := fmt.Sprintf("Forwarding %s:%d -> guest:%d added on adapter %d.", hostIP, req.HostPort, req.GuestPort, req.Slot)
	return models.NetworkOperationResponse{Success: true, VMID: id, Message: message}, nil
}

// DeletePortForwarding removes a NAT port-forwarding rule by NIC slot and name.
// Live on a running VM (`controlvm natpfN delete`), written to config otherwise
// (`modifyvm --natpfN delete`).
func (s *service) DeletePortForwarding(ctx context.Context, id string, slot int, name string) (models.NetworkOperationResponse, error) {
	if !IsValidVmID(id) {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Invalid VM identifier."}
	}
	if slot < 1 || slot > 8 {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Network adapter slot must be between 1 and 8."}
	}
	name = strings.TrimSpace(name)
	if !isPlausibleForwardingName(name) {
		return models.NetworkOperationResponse{}, &ValidationError{Message: "Rule name must be 1-64 characters, cannot contain a comma, and cannot start with a dash."}
	}

	path, err := s.resolveVBoxManage(ctx)
	if err != nil {
		s.logOperation(ctx, id, "network.forwarding.delete", false, "VirtualBox/VBoxManage not discovered.")
		return models.NetworkOperationResponse{}, err
	}

	info, err := s.readShowVmInfo(ctx, path, id, "reading VM state for port forwarding removal")
	if err != nil {
		return models.NetworkOperationResponse{}, err
	}

	live := vmStateIsLive(parseVmState(info))
	if err := s.runControlCommand(ctx, path, natpfDeleteArgs(id, slot, name, live), "removing port forwarding rule"); err != nil {
		s.logOperation(ctx, id, "network.forwarding.delete", false, "VBoxManage port forwarding remove failed.")
		return models.NetworkOperationResponse{}, err
	}

	s.logOperation(ctx, id, "network.forwarding.delete", true, "")
	return models.NetworkOperationResponse{
		Success: true,
		VMID:    id,
		Message: fmt.Sprintf("Forwarding rule %q removed from adapter %d.", name, slot),
	}, nil
}

// readShowVmInfoHuman runs `showvminfo <id>` WITHOUT --machinereadable and
// returns stdout. The human-readable form is required for NAT port-forwarding
// rules because it labels each rule with its NIC number (see nicRuleLineRe).
func (s *service) readShowVmInfoHuman(ctx context.Context, path, id, description string) (string, error) {
	result, runErr := s.runner.RunContext(ctx, path, showVmInfoHumanArgs(id), 10*time.Second)
	if runErr != nil {
		return "", &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage failed while %s: %v", description, runErr),
		}
	}
	if result.ExitCode != 0 {
		return "", &ExecutionError{
			ExitCode:      result.ExitCode,
			StandardError: result.StandardError,
			Message:       fmt.Sprintf("VBoxManage exited with code %d while %s", result.ExitCode, description),
		}
	}
	return result.StandardOutput, nil
}

func showVmInfoHumanArgs(id string) []string {
	return []string{"showvminfo", id}
}

// hostPortForwardedByOtherVM returns the name of another registered VM that
// already forwards hostPort/proto, or "" if none does. Host ports are global to
// the host, so a cross-VM collision makes one NAT listener fail to bind at
// runtime. It is best-effort: it enumerates VMs and skips any whose info cannot
// be read, and excludes excludeID (the VM being modified, already checked
// locally).
func (s *service) hostPortForwardedByOtherVM(ctx context.Context, path, excludeID, proto string, hostPort int) string {
	result, err := s.runner.RunContext(ctx, path, listVmsArgs(), 10*time.Second)
	if err != nil || result.ExitCode != 0 {
		return ""
	}
	for _, vm := range parseListVmsOutput(result.StandardOutput) {
		if vm.ID == "" || strings.EqualFold(vm.ID, excludeID) {
			continue
		}
		human, herr := s.readShowVmInfoHuman(ctx, path, vm.ID, "reading port forwarding rules of another VM")
		if herr != nil {
			continue
		}
		for _, rules := range parseForwardingRules(human) {
			for _, r := range rules {
				if r.HostPort == hostPort && strings.EqualFold(strings.TrimSpace(r.Protocol), proto) {
					return vm.Name
				}
			}
		}
	}
	return ""
}

// forwardingRuleSpec renders a rule as the VBoxManage natpf field string:
// "name,proto,hostip,hostport,guestip,guestport". Empty host/guest IPs are
// rendered as empty fields, which VirtualBox accepts.
func forwardingRuleSpec(rule models.PortForwardingRule) string {
	return fmt.Sprintf("%s,%s,%s,%d,%s,%d", rule.Name, rule.Protocol, rule.HostIP, rule.HostPort, rule.GuestIP, rule.GuestPort)
}

// natpfAddArgs builds the add command. Live uses `controlvm natpfN`; stopped
// uses `modifyvm --natpfN`.
func natpfAddArgs(id string, slot int, rule models.PortForwardingRule, live bool) []string {
	spec := forwardingRuleSpec(rule)
	n := strconv.Itoa(slot)
	if live {
		return []string{"controlvm", id, "natpf" + n, spec}
	}
	return []string{"modifyvm", id, "--natpf" + n, spec}
}

// natpfDeleteArgs builds the delete-by-name command. Live uses
// `controlvm natpfN delete`; stopped uses `modifyvm --natpfN delete`.
func natpfDeleteArgs(id string, slot int, name string, live bool) []string {
	n := strconv.Itoa(slot)
	if live {
		return []string{"controlvm", id, "natpf" + n, "delete", name}
	}
	return []string{"modifyvm", id, "--natpf" + n, "delete", name}
}

// isPlausibleForwardingName validates a port-forwarding rule name. The name is
// the first field of the comma-delimited natpf spec, so a comma is forbidden. A
// leading dash and control characters are rejected for the same defense-in-depth
// reasons as host interface names (see isPlausibleHostInterface).
func isPlausibleForwardingName(name string) bool {
	if len(name) == 0 || len(name) > 64 || name[0] == '-' {
		return false
	}
	if strings.Contains(name, ",") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
