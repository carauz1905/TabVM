package vbox

import (
	"regexp"
	"strconv"
	"strings"
)

// nicInfo describes a single virtual NIC as configured on the host, parsed from
// `VBoxManage showvminfo <id> --machinereadable`.
type nicInfo struct {
	slot int
	mode string
	mac  string
}

// parseVmResources extracts the configured CPU count and RAM (in MB) from the
// machine-readable output of `VBoxManage showvminfo <id> --machinereadable`.
// Missing or malformed values yield zero so callers can decide how to present
// an unknown resource.
func parseVmResources(output string) (cpuCount int, ramMB int) {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		switch key {
		case "cpus":
			if n, err := strconv.Atoi(value); err == nil {
				cpuCount = n
			}
		case "memory":
			if n, err := strconv.Atoi(value); err == nil {
				ramMB = n
			}
		}
	}
	return cpuCount, ramMB
}

// parseVmNICs extracts the configured virtual NICs (their slot, connection mode,
// and MAC address) from machine-readable showvminfo output. Adapters whose mode
// is "none" are omitted because they are not wired to any network.
func parseVmNICs(output string) []nicInfo {
	modes := map[int]string{}
	macs := map[int]string{}
	maxSlot := 0

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadable(line)
		if !ok {
			continue
		}
		if slot, isNIC := slotSuffix(key, "nic"); isNIC {
			modes[slot] = strings.ToLower(value)
			if slot > maxSlot {
				maxSlot = slot
			}
			continue
		}
		if slot, isMAC := slotSuffix(key, "macaddress"); isMAC {
			macs[slot] = normalizeMAC(value)
			if slot > maxSlot {
				maxSlot = slot
			}
		}
	}

	nics := make([]nicInfo, 0, len(modes))
	for slot := 1; slot <= maxSlot; slot++ {
		mode, ok := modes[slot]
		if !ok || mode == "" || mode == "none" {
			continue
		}
		nics = append(nics, nicInfo{slot: slot, mode: mode, mac: macs[slot]})
	}
	return nics
}

// parseGuestNetworks extracts guest-reported IPv4 addresses from the output of
// `VBoxManage guestproperty enumerate <id> --patterns "/VirtualBox/GuestInfo/*"`.
// It returns whether any GuestInfo property was present (a proxy for Guest
// Additions actively reporting) and a map keyed by normalized MAC address so the
// caller can attach each address to the matching configured NIC. Guest network
// indices do not necessarily match host NIC slots, so correlation is by MAC.
func parseGuestNetworks(output string) (gaPresent bool, ipv4ByMAC map[string][]string) {
	ipsByIndex := map[string][]string{}
	macByIndex := map[string]string{}

	for _, line := range strings.Split(output, "\n") {
		name, value, ok := splitGuestProperty(line)
		if !ok {
			continue
		}
		if !strings.HasPrefix(name, "/VirtualBox/GuestInfo/") {
			continue
		}
		gaPresent = true

		rest := strings.TrimPrefix(name, "/VirtualBox/GuestInfo/Net/")
		if rest == name {
			continue // not a Net/* property
		}
		segments := strings.Split(rest, "/")
		// Expected forms: "<idx>/V4/IP", "<idx>/MAC".
		if len(segments) == 3 && segments[1] == "V4" && segments[2] == "IP" {
			if ip := strings.TrimSpace(value); ip != "" && ip != "0.0.0.0" {
				ipsByIndex[segments[0]] = append(ipsByIndex[segments[0]], ip)
			}
		} else if len(segments) == 2 && segments[1] == "MAC" {
			macByIndex[segments[0]] = normalizeMAC(value)
		}
	}

	ipv4ByMAC = map[string][]string{}
	for idx, ips := range ipsByIndex {
		mac := macByIndex[idx]
		if mac == "" {
			continue
		}
		ipv4ByMAC[mac] = append(ipv4ByMAC[mac], ips...)
	}
	return gaPresent, ipv4ByMAC
}

// splitMachineReadable parses one `key="value"` line from --machinereadable
// output, lowercasing the key and stripping surrounding quotes from the value.
func splitMachineReadable(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key = strings.ToLower(strings.TrimSpace(parts[0]))
	value = strings.Trim(strings.TrimSpace(parts[1]), `"`)
	return key, value, true
}

// splitGuestProperty parses one line of `guestproperty enumerate` output of the
// form: `Name: <name>, value: <value>, timestamp: <ts>, flags: <flags>`.
func splitGuestProperty(line string) (name, value string, ok bool) {
	line = strings.TrimSpace(line)

	// Verbose form (plain `guestproperty enumerate`):
	//   "Name: <name>, value: <value>, timestamp: ..., flags: ..."
	const namePrefix = "Name: "
	if strings.HasPrefix(line, namePrefix) {
		rest := strings.TrimPrefix(line, namePrefix)
		const valueSep = ", value: "
		sepIdx := strings.Index(rest, valueSep)
		if sepIdx < 0 {
			return "", "", false
		}
		name = strings.TrimSpace(rest[:sepIdx])
		rest = rest[sepIdx+len(valueSep):]
		if commaIdx := strings.Index(rest, ", timestamp:"); commaIdx >= 0 {
			rest = rest[:commaIdx]
		}
		return name, strings.TrimSpace(rest), true
	}

	// Terse form (`enumerate --patterns`, the default on VBox 7.x):
	//   "<name>   = 'value'   @ <timestamp> <flags>"
	if eqIdx := strings.Index(line, " = "); eqIdx >= 0 {
		name = strings.TrimSpace(line[:eqIdx])
		rest := line[eqIdx+len(" = "):]
		if len(rest) > 0 && rest[0] == '\'' {
			if closeIdx := strings.Index(rest[1:], "'"); closeIdx >= 0 {
				return name, rest[1 : 1+closeIdx], true
			}
		}
	}
	return "", "", false
}

// slotSuffix reports whether key is prefix followed by a positive integer slot
// (e.g. "nic1", "macaddress2") and returns that slot number.
func slotSuffix(key, prefix string) (int, bool) {
	if !strings.HasPrefix(key, prefix) {
		return 0, false
	}
	suffix := key[len(prefix):]
	if suffix == "" {
		return 0, false
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// normalizeMAC strips common separators and uppercases a MAC address so values
// from showvminfo (e.g. "080027ABCDEF") and guestproperty compare equal.
func normalizeMAC(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer(":", "", "-", "", ".", "", " ", "")
	return strings.ToUpper(replacer.Replace(s))
}

// diskAttachment is a hard-disk medium attached to a VM, parsed from showvminfo.
type diskAttachment struct {
	name string
	uuid string
}

var (
	imageUUIDKeyRe  = regexp.MustCompile(`^(.*)-ImageUUID-(\d+)-(\d+)$`)
	attachmentKeyRe = regexp.MustCompile(`^(.*)-(\d+)-(\d+)$`)
	diskImageExts   = []string{".vdi", ".vmdk", ".vhd", ".hdd"}
)

// parseDiskAttachments extracts attached hard-disk media (name and medium UUID)
// from machine-readable showvminfo output. DVD/ISO and empty drives are skipped
// so only real VM disks are reported. The medium UUID lets the caller query
// per-disk capacity and allocation with showmediuminfo.
func parseDiskAttachments(output string) []diskAttachment {
	paths := map[string]string{}
	uuids := map[string]string{}
	order := []string{}

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := splitMachineReadableRawKey(line)
		if !ok {
			continue
		}
		if m := imageUUIDKeyRe.FindStringSubmatch(key); m != nil {
			uuids[m[1]+"|"+m[2]+"|"+m[3]] = value
			continue
		}
		if m := attachmentKeyRe.FindStringSubmatch(key); m != nil {
			if !hasDiskImageExt(value) {
				continue
			}
			slot := m[1] + "|" + m[2] + "|" + m[3]
			if _, seen := paths[slot]; !seen {
				order = append(order, slot)
			}
			paths[slot] = value
		}
	}

	disks := make([]diskAttachment, 0, len(order))
	for _, slot := range order {
		disks = append(disks, diskAttachment{name: baseName(paths[slot]), uuid: uuids[slot]})
	}
	return disks
}

// parseMediumInfo extracts the virtual capacity and the actual host-side
// allocation of a medium from `VBoxManage showmediuminfo <uuid>` output, both in
// bytes.
func parseMediumInfo(output string) (capacityBytes int64, allocatedBytes int64) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := afterLabel(line, "Capacity:"); ok {
			capacityBytes = parseSizeWithUnit(v)
		} else if v, ok := afterLabel(line, "Size on disk:"); ok {
			allocatedBytes = parseSizeWithUnit(v)
		}
	}
	return capacityBytes, allocatedBytes
}

// parseSizeWithUnit converts a VBoxManage size string such as "51200 MBytes"
// into bytes. A bare number is treated as bytes.
func parseSizeWithUnit(s string) int64 {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	mult := float64(1)
	if len(fields) >= 2 {
		switch strings.ToLower(fields[1]) {
		case "kbytes", "kb":
			mult = 1 << 10
		case "mbytes", "mb":
			mult = 1 << 20
		case "gbytes", "gb":
			mult = 1 << 30
		case "tbytes", "tb":
			mult = 1 << 40
		}
	}
	return int64(n * mult)
}

// splitMachineReadableRawKey is like splitMachineReadable but preserves the key
// case and strips surrounding quotes from the key, which storage-attachment keys
// (e.g. `"SATA-0-0"`) carry.
func splitMachineReadableRawKey(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key = strings.Trim(strings.TrimSpace(parts[0]), `"`)
	value = strings.Trim(strings.TrimSpace(parts[1]), `"`)
	return key, value, true
}

func hasDiskImageExt(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range diskImageExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// baseName returns the final path element, splitting on both / and \ so Windows
// medium paths parse the same regardless of the host running the tests.
func baseName(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func afterLabel(line, label string) (string, bool) {
	if !strings.HasPrefix(line, label) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, label)), true
}
