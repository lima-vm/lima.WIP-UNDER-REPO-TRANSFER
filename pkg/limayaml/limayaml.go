package limayaml

import (
	"net"

	"github.com/opencontainers/go-digest"
)

type LimaYAML struct {
	Arch         *Arch            `yaml:"arch,omitempty" json:"arch,omitempty"`
	Images       []Image          `yaml:"images" json:"images"` // REQUIRED
	CPUType      map[Arch]*string `yaml:"cpuType,omitempty" json:"cpuType,omitempty"`
	CPUs         *int             `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	Memory       *string          `yaml:"memory,omitempty" json:"memory,omitempty"` // go-units.RAMInBytes
	Disk         *string          `yaml:"disk,omitempty" json:"disk,omitempty"`     // go-units.RAMInBytes
	Mounts       []Mount          `yaml:"mounts,omitempty" json:"mounts,omitempty"`
	MountType    *MountType       `yaml:"mountType,omitempty" json:"mountType,omitempty"`
	SSH          SSH              `yaml:"ssh,omitempty" json:"ssh,omitempty"` // REQUIRED (FIXME)
	Firmware     Firmware         `yaml:"firmware,omitempty" json:"firmware,omitempty"`
	Video        Video            `yaml:"video,omitempty" json:"video,omitempty"`
	Provision    []Provision      `yaml:"provision,omitempty" json:"provision,omitempty"`
	Containerd   Containerd       `yaml:"containerd,omitempty" json:"containerd,omitempty"`
	Probes       []Probe          `yaml:"probes,omitempty" json:"probes,omitempty"`
	PortForwards []PortForward    `yaml:"portForwards,omitempty" json:"portForwards,omitempty"`
	Message      string           `yaml:"message,omitempty" json:"message,omitempty"`
	Networks     []Network        `yaml:"networks,omitempty" json:"networks,omitempty"`
	// `network` was deprecated in Lima v0.7.0, removed in Lima v1.0.0. Use `networks` instead.
	Env          map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	DNS          []net.IP          `yaml:"dns,omitempty" json:"dns,omitempty"`
	HostResolver HostResolver      `yaml:"hostResolver,omitempty" json:"hostResolver,omitempty"`
	// `useHostResolver` was deprecated in Lima v0.8.1, removed in Lima v1.0.0. Use `hostResolver.enabled` instead.
	PropagateProxyEnv *bool          `yaml:"propagateProxyEnv,omitempty" json:"propagateProxyEnv,omitempty"`
	CACertificates    CACertificates `yaml:"caCerts,omitempty" json:"caCerts,omitempty"`
}

type Arch = string
type MountType = string

const (
	X8664   Arch = "x86_64"
	AARCH64 Arch = "aarch64"
	RISCV64 Arch = "riscv64"

	REVSSHFS MountType = "reverse-sshfs"
	NINEP    MountType = "9p"
)

type File struct {
	Location string        `yaml:"location" json:"location"` // REQUIRED
	Arch     Arch          `yaml:"arch,omitempty" json:"arch,omitempty"`
	Digest   digest.Digest `yaml:"digest,omitempty" json:"digest,omitempty"`
}

type Kernel struct {
	File    `yaml:",inline"`
	Cmdline string `yaml:"cmdline,omitempty" json:"cmdline,omitempty"`
}

type Image struct {
	File   `yaml:",inline"`
	Kernel *Kernel `yaml:"kernel,omitempty" json:"kernel,omitempty"`
	Initrd *File   `yaml:"initrd,omitempty" json:"initrd,omitempty"`
}

type Mount struct {
	Location   string `yaml:"location" json:"location"` // REQUIRED
	MountPoint string `yaml:"mountPoint,omitempty" json:"mountPoint,omitempty"`
	Writable   *bool  `yaml:"writable,omitempty" json:"writable,omitempty"`
	SSHFS      SSHFS  `yaml:"sshfs,omitempty" json:"sshfs,omitempty"`
	NineP      NineP  `yaml:"9p,omitempty" json:"9p,omitempty"`
}

type SFTPDriver = string

const (
	SFTPDriverBuiltin           = "builtin"
	SFTPDriverOpenSSHSFTPServer = "openssh-sftp-server"
)

type SSHFS struct {
	Cache          *bool       `yaml:"cache,omitempty" json:"cache,omitempty"`
	FollowSymlinks *bool       `yaml:"followSymlinks,omitempty" json:"followSymlinks,omitempty"`
	SFTPDriver     *SFTPDriver `yaml:"sftpDriver,omitempty" json:"sftpDriver,omitempty"`
}

type NineP struct {
	SecurityModel   *string `yaml:"securityModel,omitempty" json:"securityModel,omitempty"`
	ProtocolVersion *string `yaml:"protocolVersion,omitempty" json:"protocolVersion,omitempty"`
	Msize           *string `yaml:"msize,omitempty" json:"msize,omitempty"`
	Cache           *string `yaml:"cache,omitempty" json:"cache,omitempty"`
}

type SSH struct {
	LocalPort *int `yaml:"localPort,omitempty" json:"localPort,omitempty"`

	// LoadDotSSHPubKeys loads ~/.ssh/*.pub in addition to $LIMA_HOME/_config/user.pub .
	LoadDotSSHPubKeys *bool `yaml:"loadDotSSHPubKeys,omitempty" json:"loadDotSSHPubKeys,omitempty"` // default: true
	ForwardAgent      *bool `yaml:"forwardAgent,omitempty" json:"forwardAgent,omitempty"`           // default: false
	ForwardX11        *bool `yaml:"forwardX11,omitempty" json:"forwardX11,omitempty"`               // default: false
	ForwardX11Trusted *bool `yaml:"forwardX11Trusted,omitempty" json:"forwardX11Trusted,omitempty"` // default: false
}

type Firmware struct {
	// LegacyBIOS disables UEFI if set.
	// LegacyBIOS is ignored for aarch64.
	LegacyBIOS *bool `yaml:"legacyBIOS,omitempty" json:"legacyBIOS,omitempty"`
}

type Video struct {
	// Display is a QEMU display string
	Display *string `yaml:"display,omitempty" json:"display,omitempty"`
}

type ProvisionMode = string

const (
	ProvisionModeSystem ProvisionMode = "system"
	ProvisionModeUser   ProvisionMode = "user"
)

type Provision struct {
	Mode   ProvisionMode `yaml:"mode" json:"mode"` // default: "system"
	Script string        `yaml:"script" json:"script"`
}

type Containerd struct {
	System   *bool  `yaml:"system,omitempty" json:"system,omitempty"`     // default: false
	User     *bool  `yaml:"user,omitempty" json:"user,omitempty"`         // default: true
	Archives []File `yaml:"archives,omitempty" json:"archives,omitempty"` // default: see defaultContainerdArchives
}

type ProbeMode = string

const (
	ProbeModeReadiness ProbeMode = "readiness"
)

type Probe struct {
	Mode        ProbeMode // default: "readiness"
	Description string
	Script      string
	Hint        string
}

type Proto = string

const (
	TCP Proto = "tcp"
)

type PortForward struct {
	GuestIPMustBeZero bool   `yaml:"guestIPMustBeZero,omitempty" json:"guestIPMustBeZero,omitempty"`
	GuestIP           net.IP `yaml:"guestIP,omitempty" json:"guestIP,omitempty"`
	GuestPort         int    `yaml:"guestPort,omitempty" json:"guestPort,omitempty"`
	GuestPortRange    [2]int `yaml:"guestPortRange,omitempty" json:"guestPortRange,omitempty"`
	GuestSocket       string `yaml:"guestSocket,omitempty" json:"guestSocket,omitempty"`
	HostIP            net.IP `yaml:"hostIP,omitempty" json:"hostIP,omitempty"`
	HostPort          int    `yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	HostPortRange     [2]int `yaml:"hostPortRange,omitempty" json:"hostPortRange,omitempty"`
	HostSocket        string `yaml:"hostSocket,omitempty" json:"hostSocket,omitempty"`
	Proto             Proto  `yaml:"proto,omitempty" json:"proto,omitempty"`
	Reverse           bool   `yaml:"reverse,omitempty" json:"reverse,omitempty"`
	Ignore            bool   `yaml:"ignore,omitempty" json:"ignore,omitempty"`
}

type Network struct {
	// `Lima`, `Socket`, and `VNL` are mutually exclusive; exactly one is required
	Lima string `yaml:"lima,omitempty" json:"lima,omitempty"`
	// Socket is a QEMU-compatible socket
	Socket string `yaml:"socket,omitempty" json:"socket,omitempty"`
	// VNLDeprecated is a Virtual Network Locator (https://github.com/rd235/vdeplug4/commit/089984200f447abb0e825eb45548b781ba1ebccd).
	// On macOS, only VDE2-compatible form (optionally with vde:// prefix) is supported.
	// VNLDeprecated is deprecated. Use Socket.
	VNLDeprecated        string `yaml:"vnl,omitempty" json:"vnl,omitempty"`
	SwitchPortDeprecated uint16 `yaml:"switchPort,omitempty" json:"switchPort,omitempty"` // VDE Switch port, not TCP/UDP port (only used by VDE networking)
	MACAddress           string `yaml:"macAddress,omitempty" json:"macAddress,omitempty"`
	Interface            string `yaml:"interface,omitempty" json:"interface,omitempty"`
}

type HostResolver struct {
	Enabled *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	IPv6    *bool             `yaml:"ipv6,omitempty" json:"ipv6,omitempty"`
	Hosts   map[string]string `yaml:"hosts,omitempty" json:"hosts,omitempty"`
}

type CACertificates struct {
	RemoveDefaults *bool    `yaml:"removeDefaults,omitempty" json:"removeDefaults,omitempty"` // default: false
	Files          []string `yaml:"files,omitempty" json:"files,omitempty"`
	Certs          []string `yaml:"certs,omitempty" json:"certs,omitempty"`
}

// DEPRECATED types below

// Types have been renamed to turn all references to the old names into compiler errors,
// and to avoid accidental usage in new code.

type VDEDeprecated struct {
	VNL        string `yaml:"vnl,omitempty" json:"vnl,omitempty"`
	SwitchPort uint16 `yaml:"switchPort,omitempty" json:"switchPort,omitempty"` // VDE Switch port, not TCP/UDP port
	MACAddress string `yaml:"macAddress,omitempty" json:"macAddress,omitempty"`
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
}
