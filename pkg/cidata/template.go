package cidata

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/lima-vm/lima/pkg/iso9660util"

	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/textutil"
)

//go:embed cidata.TEMPLATE.d
var templateFS embed.FS

const templateFSRoot = "cidata.TEMPLATE.d"

type CACerts struct {
	RemoveDefaults *bool
	Trusted        []Cert
}

type Cert struct {
	Lines []string
}

type Containerd struct {
	System bool
	User   bool
}
type Network struct {
	MACAddress string
	Interface  string
}
type Mount struct {
	Tag        string
	MountPoint string // abs path, accessible by the User
	Type       string
	Options    string
}
type BootCmds struct {
	Lines []string
}
type Disk struct {
	Name   string
	Device string
	Format bool
	FSType string
	FSArgs []string
}
type TemplateArgs struct {
	Name                            string // instance name
	IID                             string // instance id
	User                            string // user name
	Home                            string // home directory
	UID                             int
	SSHPubKeys                      []string
	Mounts                          []Mount
	MountType                       string
	Disks                           []Disk
	GuestInstallPrefix              string
	UpgradePackages                 bool
	Containerd                      Containerd
	Networks                        []Network
	SlirpNICName                    string
	SlirpGateway                    string
	SlirpDNS                        string
	SlirpIPAddress                  string
	UDPDNSLocalPort                 int
	TCPDNSLocalPort                 int
	HTTPProxyLocalPort              int
	HTTPProxyCACert                 Cert
	Env                             map[string]string
	Param                           map[string]string
	BootScripts                     bool
	DNSAddresses                    []string
	CACerts                         CACerts
	HostHomeMountPoint              string
	BootCmds                        []BootCmds
	RosettaEnabled                  bool
	RosettaBinFmt                   bool
	SkipDefaultDependencyResolution bool
	VMType                          string
	VSockPort                       int
	VirtioPort                      string
	Plain                           bool
	TimeZone                        string
}

func ValidateTemplateArgs(args *TemplateArgs) error {
	if err := identifiers.Validate(args.Name); err != nil {
		return err
	}
	if !osutil.ValidateUsername(args.User) {
		return errors.New("field User must be valid linux username")
	}
	if args.User == "root" {
		return errors.New("field User must not be \"root\"")
	}
	if args.UID == 0 {
		return errors.New("field UID must not be 0")
	}
	if args.Home == "" {
		return errors.New("field Home must be set")
	}
	if len(args.SSHPubKeys) == 0 {
		return errors.New("field SSHPubKeys must be set")
	}
	for i, m := range args.Mounts {
		f := m.MountPoint
		if !path.IsAbs(f) {
			return fmt.Errorf("field mounts[%d] must be absolute, got %q", i, f)
		}
	}
	return nil
}

func ExecuteTemplateCloudConfig(args *TemplateArgs) ([]byte, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	userData, err := templateFS.ReadFile(path.Join(templateFSRoot, "user-data"))
	if err != nil {
		return nil, err
	}

	cloudConfigYaml := string(userData)
	return textutil.ExecuteTemplate(cloudConfigYaml, args)
}

func ExecuteTemplateCIDataISO(args *TemplateArgs) ([]iso9660util.Entry, error) {
	if err := ValidateTemplateArgs(args); err != nil {
		return nil, err
	}

	fsys, err := fs.Sub(templateFS, templateFSRoot)
	if err != nil {
		return nil, err
	}

	var layout []iso9660util.Entry
	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("got non-regular file %q", path)
		}
		templateB, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		b, err := textutil.ExecuteTemplate(string(templateB), args)
		if err != nil {
			return err
		}
		layout = append(layout, iso9660util.Entry{
			Path:   path,
			Reader: bytes.NewReader(b),
		})
		return nil
	}

	if err := fs.WalkDir(fsys, ".", walkFn); err != nil {
		return nil, err
	}

	return layout, nil
}
