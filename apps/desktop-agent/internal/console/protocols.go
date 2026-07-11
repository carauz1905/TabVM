package console

// Protocol identifies a console protocol supported by Guacamole.
type Protocol string

const (
	// RDP is the Remote Desktop Protocol path used by VirtualBox VRDE.
	RDP Protocol = "rdp"
	// VNC is the Virtual Network Computing path, supported by Guacamole but
	// requiring guest or host-provided VNC service configuration.
	VNC Protocol = "vnc"
	// SSH is the Secure Shell terminal path, supported by Guacamole but
	// requiring guest or host-provided SSH service configuration.
	SSH Protocol = "ssh"
)

// Source identifies how a console target is produced.
type Source string

const (
	// SourceVirtualBoxVRDE indicates a target produced by VirtualBox VRDE/RDE.
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
// Only RDP through VirtualBox VRDE is auto-configured in this slice; VNC and
// SSH are Guacamole-supported but require user or guest-provided service
// configuration in a later slice.
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
			Description:      "Supported by Guacamole; requires a guest or host VNC service configured in a future slice.",
		},
		{
			ID:               SSH,
			DisplayName:      "SSH",
			CanAutoConfigure: false,
			Description:      "Supported by Guacamole; requires a guest SSH service configured in a future slice.",
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
