package limayaml

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/reflectutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
	"github.com/xorcare/pointer"
)

const (
	Default9pSecurityModel   string = "mapped-xattr"
	Default9pProtocolVersion string = "9p2000.L"
	Default9pMsize           string = "128KiB"
	Default9pCacheForRO      string = "fscache"
	Default9pCacheForRW      string = "mmap"
)

func defaultContainerdArchives() []File {
	const nerdctlVersion = "0.23.0"
	location := func(goarch string) string {
		return "https://github.com/containerd/nerdctl/releases/download/v" + nerdctlVersion + "/nerdctl-full-" + nerdctlVersion + "-linux-" + goarch + ".tar.gz"
	}
	return []File{
		{
			Location: location("amd64"),
			Arch:     X8664,
			Digest:   "sha256:2097ffb95c6ce3d847ca4882867297b5ab80e3daea6f967e96ce00cc636981b6",
		},
		{
			Location: location("arm64"),
			Arch:     AARCH64,
			Digest:   "sha256:d25171f8b6fe778b77ff0830a8e17bd61c68af69bd734fb9d7f4490e069a7816",
		},
		// No riscv64
	}
}

func MACAddress(uniqueID string) string {
	sha := sha256.Sum256([]byte(osutil.MachineID() + uniqueID))
	// "5" is the magic number in the Lima ecosystem.
	// (Visit https://en.wiktionary.org/wiki/lima and Command-F "five")
	//
	// But the second hex number is changed to 2 to satisfy the convention for
	// local MAC addresses (https://en.wikipedia.org/wiki/MAC_address#Ranges_of_group_and_locally_administered_addresses)
	//
	// See also https://gitlab.com/wireshark/wireshark/-/blob/master/manuf to confirm the uniqueness of this prefix.
	hw := append(net.HardwareAddr{0x52, 0x55, 0x55}, sha[0:3]...)
	return hw.String()
}

// builtinDefault defines the built-in default values.
var builtinDefault = &LimaYAML{
	Arch:      nil, // Resolved in FillDefault()
	Images:    nil,
	CPUType:   defaultCPUType(),
	CPUs:      pointer.Int(4),
	Memory:    pointer.String("4GiB"),
	Disk:      pointer.String("100GiB"),
	Mounts:    nil,
	MountType: pointer.String(REVSSHFS),
	Video: Video{
		Display: pointer.String("none"),
	},
	Firmware: Firmware{
		LegacyBIOS: pointer.Bool(false),
	},
	SSH: SSH{
		LocalPort:         pointer.Int(0), // Resolved by the hostagent
		LoadDotSSHPubKeys: pointer.Bool(true),
		ForwardAgent:      pointer.Bool(false),
		ForwardX11:        pointer.Bool(false),
		ForwardX11Trusted: pointer.Bool(false),
	},
	Provision: nil,
	Containerd: Containerd{
		System:   pointer.Bool(false),
		User:     pointer.Bool(true),
		Archives: defaultContainerdArchives(),
	},
	Probes:       nil,
	PortForwards: nil,
	Message:      "",
	Networks:     nil,
	Env:          nil,
	DNS:          nil,
	HostResolver: HostResolver{
		Enabled: pointer.Bool(true),
		IPv6:    pointer.Bool(false),
		Hosts:   nil,
	},
	PropagateProxyEnv: pointer.Bool(true),
	CACertificates: CACertificates{
		RemoveDefaults: pointer.Bool(false),
		Files:          nil,
		Certs:          nil,
	},
}

// FillDefault updates undefined fields in y with defaults from d (or built-in default), and overwrites with values from o.
// Both d and o may be empty.
//
// Maps (`Env`) are being merged: first populated from d, overwritten by y, and again overwritten by o.
// Slices (e.g. `Mounts`, `Provision`) are appended, starting with o, followed by y, and finally d. This
// makes sure o takes priority over y over d, in cases it matters (e.g. `PortForwards`, where the first
// matching rule terminates the search).
//
// Exceptions:
//   - Mounts are appended in d, y, o order, but "merged" when the Location matches a previous entry;
//     the highest priority Writable setting wins.
//   - Networks are appended in d, y, o order
//   - DNS are picked from the highest priority where DNS is not empty.
//   - CACertificates Files and Certs are uniquely appended in d, y, o order
func FillDefault(y, d, o *LimaYAML, filePath string) {
	bd := builtinDefault

	// *EXCEPTION*: Remove built-in containerd archives when the custom values are specified.
	if len(d.Containerd.Archives)+len(y.Containerd.Archives)+len(o.Containerd.Archives) > 0 {
		bd.Containerd.Archives = nil
	}

	// Merge bd, d, y, and o, into x.
	// y is not altered yet, and is used later for exceptional rules.
	xx, err := reflectutil.MergeMany(bd, d, y, o)
	if err != nil {
		panic(err)
	}
	x := xx.(*LimaYAML)

	// *EXCEPTION*: Mounts are appended in d, y, o order, but "merged" when the Location matches a previous entry;
	//  the highest priority Writable setting wins.

	// Combine all mounts; highest priority entry determines writable status.
	// Only works for exact matches; does not normalize case or resolve symlinks.
	mounts := make([]Mount, 0, len(d.Mounts)+len(y.Mounts)+len(o.Mounts))
	location := make(map[string]int)
	for _, mount := range append(append(d.Mounts, y.Mounts...), o.Mounts...) {
		if i, ok := location[mount.Location]; ok {
			if mount.SSHFS.Cache != nil {
				mounts[i].SSHFS.Cache = mount.SSHFS.Cache
			}
			if mount.SSHFS.FollowSymlinks != nil {
				mounts[i].SSHFS.FollowSymlinks = mount.SSHFS.FollowSymlinks
			}
			if mount.SSHFS.SFTPDriver != nil {
				mounts[i].SSHFS.SFTPDriver = mount.SSHFS.SFTPDriver
			}
			if mount.NineP.SecurityModel != nil {
				mounts[i].NineP.SecurityModel = mount.NineP.SecurityModel
			}
			if mount.NineP.ProtocolVersion != nil {
				mounts[i].NineP.ProtocolVersion = mount.NineP.ProtocolVersion
			}
			if mount.NineP.Msize != nil {
				mounts[i].NineP.Msize = mount.NineP.Msize
			}
			if mount.NineP.Cache != nil {
				mounts[i].NineP.Cache = mount.NineP.Cache
			}
			if mount.Writable != nil {
				mounts[i].Writable = mount.Writable
			}
			if mount.MountPoint != "" {
				mounts[i].MountPoint = mount.MountPoint
			}
		} else {
			location[mount.Location] = len(mounts)
			mounts = append(mounts, mount)
		}
	}
	x.Mounts = mounts

	for i := range x.Mounts {
		mount := &x.Mounts[i]
		if mount.SSHFS.Cache == nil {
			mount.SSHFS.Cache = pointer.Bool(true)
		}
		if mount.SSHFS.FollowSymlinks == nil {
			mount.SSHFS.FollowSymlinks = pointer.Bool(false)
		}
		if mount.SSHFS.SFTPDriver == nil {
			mount.SSHFS.SFTPDriver = pointer.String("")
		}
		if mount.NineP.SecurityModel == nil {
			mounts[i].NineP.SecurityModel = pointer.String(Default9pSecurityModel)
		}
		if mount.NineP.ProtocolVersion == nil {
			mounts[i].NineP.ProtocolVersion = pointer.String(Default9pProtocolVersion)
		}
		if mount.NineP.Msize == nil {
			mounts[i].NineP.Msize = pointer.String(Default9pMsize)
		}
		if mount.Writable == nil {
			mount.Writable = pointer.Bool(false)
		}
		if mount.NineP.Cache == nil {
			if *mount.Writable {
				mounts[i].NineP.Cache = pointer.String(Default9pCacheForRW)
			} else {
				mounts[i].NineP.Cache = pointer.String(Default9pCacheForRO)
			}
		}
		if mount.MountPoint == "" {
			mounts[i].MountPoint = mount.Location
		}
	}

	// *EXCEPTION*: Networks are appended in d, y, o order
	networks := make([]Network, 0, len(d.Networks)+len(y.Networks)+len(o.Networks))
	iface := make(map[string]int)
	for _, nw := range append(append(d.Networks, y.Networks...), o.Networks...) {
		if i, ok := iface[nw.Interface]; ok {
			if nw.VNLDeprecated != "" {
				networks[i].VNLDeprecated = nw.VNLDeprecated
				networks[i].SwitchPortDeprecated = nw.SwitchPortDeprecated
				networks[i].Socket = ""
				networks[i].Lima = ""
			}
			if nw.Socket != "" {
				if nw.VNLDeprecated != "" {
					// We can't return an error, so just log it, and prefer `socket` over `vnl`
					logrus.Errorf("Network %q has both vnl=%q and socket=%q fields; ignoring vnl",
						nw.Interface, nw.VNLDeprecated, nw.Socket)
				}
				networks[i].Socket = nw.Socket
				networks[i].VNLDeprecated = ""
				networks[i].SwitchPortDeprecated = 0
				networks[i].Lima = ""
			}
			if nw.Lima != "" {
				if nw.VNLDeprecated != "" {
					// We can't return an error, so just log it, and prefer `lima` over `vnl`
					logrus.Errorf("Network %q has both vnl=%q and lima=%q fields; ignoring vnl",
						nw.Interface, nw.VNLDeprecated, nw.Lima)
				}
				if nw.Socket != "" {
					// We can't return an error, so just log it, and prefer `lima` over `socket`
					logrus.Errorf("Network %q has both socket=%q and lima=%q fields; ignoring socket",
						nw.Interface, nw.Socket, nw.Lima)
				}
				networks[i].Lima = nw.Lima
				networks[i].Socket = ""
				networks[i].VNLDeprecated = ""
				networks[i].SwitchPortDeprecated = 0
			}
			if nw.MACAddress != "" {
				networks[i].MACAddress = nw.MACAddress
			}
		} else {
			// unnamed network definitions are not combined/overwritten
			if nw.Interface != "" {
				iface[nw.Interface] = len(networks)
			}
			networks = append(networks, nw)
		}
	}
	x.Networks = networks
	for i := range x.Networks {
		nw := &x.Networks[i]
		if nw.MACAddress == "" {
			// every interface in every limayaml file must get its own unique MAC address
			nw.MACAddress = MACAddress(fmt.Sprintf("%s#%d", filePath, i))
		}
		if nw.Interface == "" {
			nw.Interface = "lima" + strconv.Itoa(i)
		}
	}

	// *EXCEPTION*: DNS are picked from the highest priority where DNS is not empty.
	// Note: DNS lists are not combined; highest priority setting is picked
	dns := y.DNS
	if len(dns) == 0 {
		dns = d.DNS
	}
	if len(o.DNS) > 0 {
		dns = o.DNS
	}
	x.DNS = dns

	// *EXCEPTION*: CACertificates Files and Certs are uniquely appended in d, y, o order
	x.CACertificates.Files = unique(append(append(d.CACertificates.Files, y.CACertificates.Files...), o.CACertificates.Files...))
	x.CACertificates.Certs = unique(append(append(d.CACertificates.Certs, y.CACertificates.Certs...), o.CACertificates.Certs...))

	// Fix up other fields
	instDir := filepath.Dir(filePath)
	fixUp(x, instDir)

	// Return the result x as y
	*y = *x
}

func fixUp(x *LimaYAML, instDir string) {
	// Resolve the default arch
	x.Arch = pointer.String(ResolveArch(x.Arch))
	for i := range x.Images {
		img := &x.Images[i]
		if img.Arch == "" {
			img.Arch = *x.Arch
		}
		if img.Kernel != nil && img.Kernel.Arch == "" {
			img.Kernel.Arch = img.Arch
		}
		if img.Initrd != nil && img.Initrd.Arch == "" {
			img.Initrd.Arch = img.Arch
		}
	}
	for i := range x.Containerd.Archives {
		f := &x.Containerd.Archives[i]
		if f.Arch == "" {
			f.Arch = *x.Arch
		}
	}

	// Resolve the default provision mode
	for i := range x.Provision {
		provision := &x.Provision[i]
		if provision.Mode == "" {
			provision.Mode = ProvisionModeSystem
		}
	}

	// Resolve the default probe mode
	for i := range x.Probes {
		probe := &x.Probes[i]
		if probe.Mode == "" {
			probe.Mode = ProbeModeReadiness
		}
		if probe.Description == "" {
			probe.Description = fmt.Sprintf("user probe %d/%d", i+1, len(x.Probes))
		}
	}

	// Fill port forward defaults
	for i := range x.PortForwards {
		FillPortForwardDefaults(&x.PortForwards[i], instDir)
		// After defaults processing the singular HostPort and GuestPort values should not be used again.
	}

	// Fix up the host resolver.
	// Values can be either names or IP addresses. Name values are canonicalized in the hostResolver.
	hosts := make(map[string]string)
	for k, v := range x.HostResolver.Hosts {
		hosts[Cname(k)] = v
	}
	x.HostResolver.Hosts = hosts
}

func FillPortForwardDefaults(rule *PortForward, instDir string) {
	if rule.Proto == "" {
		rule.Proto = TCP
	}
	if rule.GuestIP == nil {
		if rule.GuestIPMustBeZero {
			rule.GuestIP = net.IPv4zero
		} else {
			rule.GuestIP = api.IPv4loopback1
		}
	}
	if rule.HostIP == nil {
		rule.HostIP = api.IPv4loopback1
	}
	if rule.GuestPortRange[0] == 0 && rule.GuestPortRange[1] == 0 {
		if rule.GuestPort == 0 {
			rule.GuestPortRange[0] = 1
			rule.GuestPortRange[1] = 65535
		} else {
			rule.GuestPortRange[0] = rule.GuestPort
			rule.GuestPortRange[1] = rule.GuestPort
		}
	}
	if rule.HostPortRange[0] == 0 && rule.HostPortRange[1] == 0 {
		if rule.HostPort == 0 {
			rule.HostPortRange = rule.GuestPortRange
		} else {
			rule.HostPortRange[0] = rule.HostPort
			rule.HostPortRange[1] = rule.HostPort
		}
	}
	if rule.GuestSocket != "" {
		tmpl, err := template.New("").Parse(rule.GuestSocket)
		if err == nil {
			user, _ := osutil.LimaUser(false)
			data := map[string]string{
				"Home": fmt.Sprintf("/home/%s.linux", user.Username),
				"UID":  user.Uid,
				"User": user.Username,
			}
			var out bytes.Buffer
			if err := tmpl.Execute(&out, data); err == nil {
				rule.GuestSocket = out.String()
			} else {
				logrus.WithError(err).Warnf("Couldn't process guestSocket %q as a template", rule.GuestSocket)
			}
		}
	}
	if rule.HostSocket != "" {
		tmpl, err := template.New("").Parse(rule.HostSocket)
		if err == nil {
			user, _ := osutil.LimaUser(false)
			home, _ := os.UserHomeDir()
			limaHome, _ := dirnames.LimaDir()
			data := map[string]string{
				"Dir":  instDir,
				"Home": home,
				"Name": filepath.Base(instDir),
				"UID":  user.Uid,
				"User": user.Username,

				"Instance": filepath.Base(instDir), // DEPRECATED, use `{{.Name}}`
				"LimaHome": limaHome,               // DEPRECATED, (use `Dir` instead of `{{.LimaHome}}/{{.Instance}}`
			}
			var out bytes.Buffer
			if err := tmpl.Execute(&out, data); err == nil {
				rule.HostSocket = out.String()
			} else {
				logrus.WithError(err).Warnf("Couldn't process hostSocket %q as a template", rule.HostSocket)
			}
		}
		if !filepath.IsAbs(rule.HostSocket) {
			rule.HostSocket = filepath.Join(instDir, filenames.SocketDir, rule.HostSocket)
		}
	}
}

func NewArch(arch string) Arch {
	switch arch {
	case "amd64":
		return X8664
	case "arm64":
		return AARCH64
	case "riscv64":
		return RISCV64
	default:
		logrus.Warnf("Unknown arch: %s", arch)
		return arch
	}
}

func ResolveArch(s *string) Arch {
	if s == nil || *s == "" || *s == "default" {
		return NewArch(runtime.GOARCH)
	}
	return *s
}

func IsAccelOS() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "netbsd", "windows":
		// Accelerator
		return true
	}
	// Using TCG
	return false
}

func HasHostCPU() bool {
	switch runtime.GOOS {
	case "darwin", "linux":
		return true
	case "netbsd", "windows":
		return false
	}
	// Not reached
	return false
}

func HasMaxCPU() bool {
	// WHPX: Unexpected VP exit code 4
	return runtime.GOOS != "windows"
}

func IsNativeArch(arch Arch) bool {
	nativeX8664 := arch == X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == AARCH64 && runtime.GOARCH == "arm64"
	nativeRISCV64 := arch == RISCV64 && runtime.GOARCH == "riscv64"
	return nativeX8664 || nativeAARCH64 || nativeRISCV64
}

func Cname(host string) string {
	host = strings.ToLower(host)
	if !strings.HasSuffix(host, ".") {
		host += "."
	}
	return host
}

func unique(s []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range s {
		if _, found := keys[entry]; !found {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func defaultCPUType() map[Arch]*string {
	cpuType := map[Arch]*string{
		AARCH64: pointer.String("cortex-a72"),
		// Since https://github.com/lima-vm/lima/pull/494, we use qemu64 cpu for better emulation of x86_64.
		X8664:   pointer.String("qemu64"),
		RISCV64: pointer.String("rv64"),
	}
	for arch := range cpuType {
		if IsNativeArch(arch) && IsAccelOS() {
			if HasHostCPU() {
				cpuType[arch] = pointer.String("host")
			} else if HasMaxCPU() {
				cpuType[arch] = pointer.String("max")
			}
		}
	}
	return cpuType
}
