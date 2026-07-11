package console

// Protocol identifies a console protocol the agent can model.
type Protocol string

const (
	// RDP is the Remote Desktop Protocol path used by VirtualBox VRDE.
	RDP Protocol = "rdp"
	// VNC is the Virtual Network Computing path, requiring a guest or
	// host-provided VNC service.
	VNC Protocol = "vnc"
	// SSH is the Secure Shell terminal path, requiring a guest or host-provided
	// SSH service.
	SSH Protocol = "ssh"
)

// Source identifies how a console target is produced.
type Source string

const (
	// SourceVirtualBoxVRDE indicates a target produced by VirtualBox VRDE/RDP.
	SourceVirtualBoxVRDE Source = "virtualbox-vrde"
)

// Capability describes a supported console protocol and what the agent can
// currently do with it.
type Capability struct {
	ID               Protocol `json:"id"`
	DisplayName      string   `json:"displayName"`
	CanAutoConfigure bool     `json:"canAutoConfigure"`
	Description      string   `json:"description"`
}

// Capabilities returns the protocol-aware console capabilities for the agent.
// Only RDP through VirtualBox VRDE is auto-configured; VNC and SSH require a
// user, guest, or host-provided service and are exposed as capability metadata.
func Capabilities() []Capability {
	return []Capability{
		{
			ID:               RDP,
			DisplayName:      "RDP",
			CanAutoConfigure: true,
			Description:      "Auto-configured through VirtualBox VRDE on the loopback interface.",
		},
		{
			ID:               VNC,
			DisplayName:      "VNC",
			CanAutoConfigure: false,
			Description:      "Requires a guest or host VNC service; not auto-configured.",
		},
		{
			ID:               SSH,
			DisplayName:      "SSH",
			CanAutoConfigure: false,
			Description:      "Requires a guest or host SSH service; not auto-configured.",
		},
	}
}

// Target is a single protocol-capable console endpoint.
type Target struct {
	Protocol    Protocol `json:"protocol"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Source      Source   `json:"source"`
	DisplayName string   `json:"displayName"`
	Ready       bool     `json:"ready"`
}
