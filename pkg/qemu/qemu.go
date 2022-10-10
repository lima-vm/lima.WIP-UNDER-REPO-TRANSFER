package qemu

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/networks"
	qemu "github.com/lima-vm/lima/pkg/qemu/const"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Name         string
	InstanceDir  string
	LimaYAML     *limayaml.LimaYAML
	SSHLocalPort int
}

func downloadFile(dest string, f limayaml.File, description string, expectedArch limayaml.Arch) error {
	if f.Arch != expectedArch {
		return fmt.Errorf("unsupported arch: %q", f.Arch)
	}
	logrus.WithField("digest", f.Digest).Infof("Attempting to download %s from %q", description, f.Location)
	res, err := downloader.Download(dest, f.Location,
		downloader.WithCache(),
		downloader.WithExpectedDigest(f.Digest),
	)
	if err != nil {
		return fmt.Errorf("failed to download %q: %w", f.Location, err)
	}
	logrus.Debugf("res.ValidatedDigest=%v", res.ValidatedDigest)
	switch res.Status {
	case downloader.StatusDownloaded:
		logrus.Infof("Downloaded %s from %q", description, f.Location)
	case downloader.StatusUsedCache:
		logrus.Infof("Using cache %q", res.CachePath)
	default:
		logrus.Warnf("Unexpected result from downloader.Download(): %+v", res)
	}
	return nil
}

// EnsureDisk also ensures the kernel and the initrd
func EnsureDisk(cfg Config) error {
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	baseDisk := filepath.Join(cfg.InstanceDir, filenames.BaseDisk)
	kernel := filepath.Join(cfg.InstanceDir, filenames.Kernel)
	kernelCmdline := filepath.Join(cfg.InstanceDir, filenames.KernelCmdline)
	initrd := filepath.Join(cfg.InstanceDir, filenames.Initrd)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(cfg.LimaYAML.Images))
		for i, f := range cfg.LimaYAML.Images {
			if err := downloadFile(baseDisk, f.File, "the image", *cfg.LimaYAML.Arch); err != nil {
				errs[i] = err
				continue
			}
			if f.Kernel != nil {
				if err := downloadFile(kernel, f.Kernel.File, "the kernel", *cfg.LimaYAML.Arch); err != nil {
					errs[i] = err
					continue
				}
				if f.Kernel.Cmdline != "" {
					if err := os.WriteFile(kernelCmdline, []byte(f.Kernel.Cmdline), 0644); err != nil {
						errs[i] = err
						continue
					}
				}
			}
			if f.Initrd != nil {
				if err := downloadFile(initrd, *f.Initrd, "the initrd", *cfg.LimaYAML.Arch); err != nil {
					errs[i] = err
					continue
				}
			}
			ensuredBaseDisk = true
			break
		}
		if !ensuredBaseDisk {
			return fmt.Errorf("failed to download the image, attempted %d candidates, errors=%v",
				len(cfg.LimaYAML.Images), errs)
		}
	}
	diskSize, _ := units.RAMInBytes(*cfg.LimaYAML.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	args := []string{"create", "-f", "qcow2"}
	if !isBaseDiskISO {
		baseDiskFormat, err := imgutil.DetectFormat(baseDisk)
		if err != nil {
			return err
		}
		args = append(args, "-F", baseDiskFormat, "-b", baseDisk)
	}
	args = append(args, diffDisk, strconv.Itoa(int(diskSize)))
	cmd := exec.Command("qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}

func argValue(args []string, key string) (string, bool) {
	if !strings.HasPrefix(key, "-") {
		panic(fmt.Errorf("got unexpected key %q", key))
	}
	for i, s := range args {
		if s == key {
			if i == len(args)-1 {
				return "", true
			}
			value := args[i+1]
			if strings.HasPrefix(value, "-") {
				return "", true
			}
			return value, true
		}
	}
	return "", false
}

// appendArgsIfNoConflict can be used for: -cpu, -machine, -m, -boot ...
// appendArgsIfNoConflict cannot be used for: -drive, -cdrom, ...
func appendArgsIfNoConflict(args []string, k, v string) []string {
	if !strings.HasPrefix(k, "-") {
		panic(fmt.Errorf("got unexpected key %q", k))
	}
	switch k {
	case "-drive", "-cdrom", "-chardev", "-blockdev", "-netdev", "-device":
		panic(fmt.Errorf("appendArgsIfNoConflict() must not be called with k=%q", k))
	}

	if v == "" {
		if _, ok := argValue(args, k); ok {
			return args
		}
		return append(args, k)
	}

	if origV, ok := argValue(args, k); ok {
		logrus.Warnf("Not adding QEMU argument %q %q, as it conflicts with %q %q", k, v, k, origV)
		return args
	}
	return append(args, k, v)
}

type features struct {
	// AccelHelp is the output of `qemu-system-x86_64 -accel help`
	// e.g. "Accelerators supported in QEMU binary:\ntcg\nhax\nhvf\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	AccelHelp []byte
	// NetdevHelp is the output of `qemu-system-x86_64 -netdev help`
	// e.g. "Available netdev backend types:\nsocket\nhubport\ntap\nuser\nvde\nbridge\vhost-user\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	NetdevHelp []byte
	// MachineHelp is the output of `qemu-system-x86_64 -machine help`
	// e.g. "Supported machines are:\nakita...\n...virt-6.2...\n...virt-7.0...\n...\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	MachineHelp []byte

	// VersionGEQ7 is true when the QEMU version seems v7.0.0 or later
	VersionGEQ7 bool
}

func inspectFeatures(exe string) (*features, error) {
	var (
		f      features
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd := exec.Command(exe, "-M", "none", "-accel", "help")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	}
	f.AccelHelp = stdout.Bytes()
	// on older versions qemu will write "help" output to stderr
	if len(f.AccelHelp) == 0 {
		f.AccelHelp = stderr.Bytes()
	}

	cmd = exec.Command(exe, "-M", "none", "-netdev", "help")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	} else {
		f.NetdevHelp = stdout.Bytes()
		if len(f.NetdevHelp) == 0 {
			f.NetdevHelp = stderr.Bytes()
		}
	}

	cmd = exec.Command(exe, "-machine", "help")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	} else {
		f.MachineHelp = stdout.Bytes()
		if len(f.MachineHelp) == 0 {
			f.MachineHelp = stderr.Bytes()
		}
	}
	f.VersionGEQ7 = strings.Contains(string(f.MachineHelp), "-7.0")

	return &f, nil
}

// showDarwinARM64HVFQEMU620Warning shows a warning on M1 macOS when QEMU is older than 6.2.0_1.
//
// See:
// - https://gitlab.com/qemu-project/qemu/-/issues/899
// - https://github.com/Homebrew/homebrew-core/pull/96743
// - https://github.com/lima-vm/lima/issues/712
func showDarwinARM64HVFQEMU620Warning(exe, accel string, features *features) {
	if runtime.GOOS != "darwin" {
		return
	}
	if runtime.GOARCH != "arm64" {
		return
	}
	if accel != "hvf" {
		return
	}
	if features.VersionGEQ7 {
		return
	}
	if exeFull, err := exec.LookPath(exe); err == nil {
		if exeResolved, err2 := filepath.EvalSymlinks(exeFull); err2 == nil {
			if strings.Contains(exeResolved, "Cellar/qemu/6.2.0_") {
				// Homebrew's QEMU 6.2.0_1 or later
				return
			}
		}
	}
	w := "This version of QEMU might not be able to boot recent Linux guests on M1 macOS hosts."
	if _, err := exec.LookPath("brew"); err == nil {
		w += "Run `brew upgrade` and make sure your QEMU version is 6.2.0_1 or later."
	} else {
		w += `Reinstall QEMU with the following commits (included in QEMU 7.0.0):
- https://github.com/qemu/qemu/commit/ad99f64f "hvf: arm: Use macros for sysreg shift/masking"
- https://github.com/qemu/qemu/commit/7f6c295c "hvf: arm: Handle unknown ID registers as RES0"
`
		w += "See https://github.com/Homebrew/homebrew-core/pull/96743 for the further information."
	}
	logrus.Warn(w)
}

func getMacOSProductVersion() (*semver.Version, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	// output is like "12.3.1\n"
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute %v: %w", cmd.Args, err)
	}
	verTrimmed := strings.TrimSpace(string(b))
	// macOS 12.4 returns just "12.4\n"
	for strings.Count(verTrimmed, ".") < 2 {
		verTrimmed += ".0"
	}
	verSem, err := semver.NewVersion(verTrimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse macOS version %q: %w", verTrimmed, err)
	}
	return verSem, nil
}

// adjustMemBytesDarwinARM64HVF adjusts the memory to be <= 3 GiB, only when the following conditions are met:
//
// - Host OS   <  macOS 12.4
// - Host Arch == arm64
// - Accel     == hvf
// - QEMU      >= 7.0
//
// This adjustment is required for avoiding host kernel panic. The issue was fixed in macOS 12.4 Beta 1.
// See https://github.com/lima-vm/lima/issues/795 https://gitlab.com/qemu-project/qemu/-/issues/903#note_911000975
func adjustMemBytesDarwinARM64HVF(memBytes int64, accel string, features *features) int64 {
	const safeSize = 3 * 1024 * 1024 * 1024 // 3 GiB
	if memBytes <= safeSize {
		return memBytes
	}
	if runtime.GOOS != "darwin" {
		return memBytes
	}
	if runtime.GOARCH != "arm64" {
		return memBytes
	}
	if accel != "hvf" {
		return memBytes
	}
	if !features.VersionGEQ7 {
		return memBytes
	}
	macOSProductVersion, err := getMacOSProductVersion()
	if err != nil {
		logrus.Warn(err)
		return memBytes
	}
	if !macOSProductVersion.LessThan(*semver.New("12.4.0")) {
		return memBytes
	}
	logrus.Warnf("Reducing the guest memory from %s to %s, to avoid host kernel panic on macOS <= 12.3 with QEMU >= 7.0; "+
		"Please update macOS to 12.4 or later, or downgrade QEMU to 6.2; "+
		"See https://github.com/lima-vm/lima/issues/795 for the further background.",
		units.BytesSize(float64(memBytes)), units.BytesSize(float64(safeSize)))
	memBytes = safeSize
	return memBytes
}

func Cmdline(cfg Config) (string, []string, error) {
	y := cfg.LimaYAML
	exe, args, err := getExe(*y.Arch)
	if err != nil {
		return "", nil, err
	}

	features, err := inspectFeatures(exe)
	if err != nil {
		return "", nil, err
	}

	// Architecture
	accel := getAccel(*y.Arch)
	if !strings.Contains(string(features.AccelHelp), accel) {
		return "", nil, fmt.Errorf("accelerator %q is not supported by %s", accel, exe)
	}
	showDarwinARM64HVFQEMU620Warning(exe, accel, features)

	// Memory
	memBytes, err := units.RAMInBytes(*y.Memory)
	if err != nil {
		return "", nil, err
	}
	memBytes = adjustMemBytesDarwinARM64HVF(memBytes, accel, features)
	args = appendArgsIfNoConflict(args, "-m", strconv.Itoa(int(memBytes>>20)))

	// CPU
	cpu, ok := y.CPUType[*y.Arch]
	if !ok || cpu == nil {
		return "", nil, fmt.Errorf("no cpu type is defined for arch %q", *y.Arch)
	}
	args = appendArgsIfNoConflict(args, "-cpu", *cpu)
	switch *y.Arch {
	case limayaml.X8664:
		if strings.HasPrefix(*cpu, "qemu64") && runtime.GOOS != "windows" {
			// use q35 machine with vmware io port disabled.
			args = appendArgsIfNoConflict(args, "-machine", "q35,vmport=off")
			// use tcg accelerator with multi threading with 512MB translation block size
			// https://qemu-project.gitlab.io/qemu/devel/multi-thread-tcg.html?highlight=tcg
			// https://qemu-project.gitlab.io/qemu/system/invocation.html?highlight=tcg%20opts
			// this will make sure each vCPU will be backed by 1 host user thread.
			args = appendArgsIfNoConflict(args, "-accel", "tcg,thread=multi,tb-size=512")
			// This will disable CPU S3 state.
			args = append(args, "-global", "ICH9-LPC.disable_s3=1")
		} else if runtime.GOOS == "windows" && accel == "whpx" {
			// whpx: injection failed, MSI (0, 0) delivery: 0, dest_mode: 0, trigger mode: 0, vector: 0
			args = appendArgsIfNoConflict(args, "-machine", "q35,accel="+accel+",kernel-irqchip=off")
		} else {
			args = appendArgsIfNoConflict(args, "-machine", "q35,accel="+accel)
		}
	case limayaml.AARCH64:
		machine := "virt,accel=" + accel
		// QEMU >= 7.0 requires highmem=off NOT to be set, otherwise fails with "Addressing limited to 32 bits, but memory exceeds it by 1073741824 bytes"
		// QEMU <  7.0 requires highmem=off to be set, otherwise fails with "VCPU supports less PA bits (36) than requested by the memory map (40)"
		// https://github.com/lima-vm/lima/issues/680
		// https://github.com/lima-vm/lima/pull/24
		// But when the memory size is <= 3 GiB, we can always set highmem=off.
		if !features.VersionGEQ7 || memBytes <= 3*1024*1024*1024 {
			machine += ",highmem=off"
		}
		args = appendArgsIfNoConflict(args, "-machine", machine)
	case limayaml.RISCV64:
		machine := "virt,accel=" + accel
		args = appendArgsIfNoConflict(args, "-machine", machine)
	}

	// SMP
	args = appendArgsIfNoConflict(args, "-smp",
		fmt.Sprintf("%d,sockets=1,cores=%d,threads=1", *y.CPUs, *y.CPUs))

	// Firmware
	legacyBIOS := *y.Firmware.LegacyBIOS
	if legacyBIOS && *y.Arch != limayaml.X8664 {
		logrus.Warnf("field `firmware.legacyBIOS` is not supported for architecture %q, ignoring", *y.Arch)
		legacyBIOS = false
	}
	if !legacyBIOS && *y.Arch != limayaml.RISCV64 {
		firmware, err := getFirmware(exe, *y.Arch)
		if err != nil {
			return "", nil, err
		}
		args = append(args, "-drive", fmt.Sprintf("if=pflash,format=raw,readonly=on,file=%s", firmware))
	}

	// Disk
	baseDisk := filepath.Join(cfg.InstanceDir, filenames.BaseDisk)
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	isBaseDiskCDROM, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return "", nil, err
	}
	if isBaseDiskCDROM {
		args = appendArgsIfNoConflict(args, "-boot", "order=d,splash-time=0,menu=on")
		args = append(args, "-drive", fmt.Sprintf("file=%s,media=cdrom,readonly=on", baseDisk))
	} else {
		args = appendArgsIfNoConflict(args, "-boot", "order=c,splash-time=0,menu=on")
	}
	if diskSize, _ := units.RAMInBytes(*cfg.LimaYAML.Disk); diskSize > 0 {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio", diffDisk))
	} else if !isBaseDiskCDROM {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio", baseDisk))
	}
	// cloud-init
	switch *y.Arch {
	case limayaml.RISCV64:
		// -cdrom does not seem recognized for RISCV64
		args = append(args,
			"-drive", "id=cdrom0,if=none,format=raw,readonly=on,file="+filepath.Join(cfg.InstanceDir, filenames.CIDataISO),
			"-device", "virtio-scsi-pci,id=scsi0",
			"-device", "scsi-cd,bus=scsi0.0,drive=cdrom0")
	default:
		// TODO: consider using virtio cdrom for all the architectures
		args = append(args, "-cdrom", filepath.Join(cfg.InstanceDir, filenames.CIDataISO))
	}

	// Kernel
	kernel := filepath.Join(cfg.InstanceDir, filenames.Kernel)
	kernelCmdline := filepath.Join(cfg.InstanceDir, filenames.KernelCmdline)
	initrd := filepath.Join(cfg.InstanceDir, filenames.Initrd)
	if _, err := os.Stat(kernel); err == nil {
		args = appendArgsIfNoConflict(args, "-kernel", kernel)
	}
	if b, err := os.ReadFile(kernelCmdline); err == nil {
		args = appendArgsIfNoConflict(args, "-append", string(b))
	}
	if _, err := os.Stat(initrd); err == nil {
		args = appendArgsIfNoConflict(args, "-initrd", initrd)
	}

	// Network
	args = append(args, "-netdev", fmt.Sprintf("user,id=net0,net=%s,dhcpstart=%s,hostfwd=tcp:127.0.0.1:%d-:22",
		qemu.SlirpNetwork, qemu.SlirpIPAddress, cfg.SSHLocalPort))
	args = append(args, "-device", "virtio-net-pci,netdev=net0,mac="+limayaml.MACAddress(cfg.InstanceDir))
	for i, nw := range y.Networks {
		var vdeSock string
		if nw.Lima != "" {
			nwCfg, err := networks.Config()
			if err != nil {
				return "", nil, err
			}
			socketVMNetOk, err := nwCfg.IsDaemonInstalled(networks.SocketVMNet)
			if err != nil {
				return "", nil, err
			}
			if socketVMNetOk {
				logrus.Debugf("Using socketVMNet (%q)", nwCfg.Paths.SocketVMNet)
				if vdeVMNetOk, _ := nwCfg.IsDaemonInstalled(networks.VDEVMNet); vdeVMNetOk {
					logrus.Debugf("Ignoring vdeVMNet (%q), as socketVMNet (%q) is available and has higher precedence", nwCfg.Paths.VDEVMNet, nwCfg.Paths.SocketVMNet)
				}
				sock, err := networks.Sock(nw.Lima)
				if err != nil {
					return "", nil, err
				}
				args = append(args, "-netdev", fmt.Sprintf("socket,id=net%d,fd={{ fd_connect %q }}", i+1, sock))
			} else if nwCfg.Paths.VDEVMNet != "" {
				logrus.Warn("vdeVMNet is deprecated, use socketVMNet instead (See docs/network.md)")
				vdeSock, err = networks.VDESock(nw.Lima)
				if err != nil {
					return "", nil, err
				}
			}
			// TODO: should we also validate that the socket exists, or do we rely on the
			// networks reconciler to throw an error when the network cannot start?
		} else if nw.Socket != "" {
			args = append(args, "-netdev", fmt.Sprintf("socket,id=net%d,fd={{ fd_connect %q }}", i+1, nw.Socket))
		} else if nw.VNLDeprecated != "" {
			// VDE4 accepts VNL like vde:///var/run/vde.ctl as well as file path like /var/run/vde.ctl .
			// VDE2 only accepts the latter form.
			// VDE2 supports macOS but VDE4 does not yet, so we trim vde:// prefix here for VDE2 compatibility.
			vdeSock = strings.TrimPrefix(nw.VNLDeprecated, "vde://")
			if !strings.Contains(vdeSock, "://") {
				if _, err := os.Stat(vdeSock); err != nil {
					return "", nil, fmt.Errorf("cannot use VNL %q: %w", nw.VNLDeprecated, err)
				}
				// vdeSock is a directory, unless vde.SwitchPort == 65535 (PTP)
				actualSocket := filepath.Join(vdeSock, "ctl")
				if nw.SwitchPortDeprecated == 65535 { // PTP
					actualSocket = vdeSock
				}
				if st, err := os.Stat(actualSocket); err != nil {
					return "", nil, fmt.Errorf("cannot use VNL %q: failed to stat %q: %w", nw.VNLDeprecated, actualSocket, err)
				} else if st.Mode()&fs.ModeSocket == 0 {
					return "", nil, fmt.Errorf("cannot use VNL %q: %q is not a socket: %w", nw.VNLDeprecated, actualSocket, err)
				}
			}
		} else {
			return "", nil, fmt.Errorf("invalid network spec %+v", nw)
		}
		if vdeSock != "" {
			if !strings.Contains(string(features.NetdevHelp), "vde") {
				return "", nil, fmt.Errorf("netdev \"vde\" is not supported by %s ( Hint: recompile QEMU with `configure --enable-vde` )", exe)
			}
			args = append(args, "-netdev", fmt.Sprintf("vde,id=net%d,sock=%s", i+1, vdeSock))
		}
		args = append(args, "-device", fmt.Sprintf("virtio-net-pci,netdev=net%d,mac=%s", i+1, nw.MACAddress))
	}

	// virtio-rng-pci accelerates starting up the OS, according to https://wiki.gentoo.org/wiki/QEMU/Options
	args = append(args, "-device", "virtio-rng-pci")

	// Graphics
	if *y.Video.Display != "" {
		args = appendArgsIfNoConflict(args, "-display", *y.Video.Display)
	}
	switch *y.Arch {
	case limayaml.X8664:
		args = append(args, "-device", "virtio-vga")
		args = append(args, "-device", "virtio-keyboard-pci")
		args = append(args, "-device", "virtio-mouse-pci")
		args = append(args, "-device", "qemu-xhci,id=usb-bus")
	default:
		// QEMU does not seem to support virtio-vga for aarch64
		args = append(args, "-vga", "none", "-device", "ramfb")
		args = append(args, "-device", "qemu-xhci,id=usb-bus")
		args = append(args, "-device", "usb-kbd,bus=usb-bus.0")
		args = append(args, "-device", "usb-mouse,bus=usb-bus.0")
	}

	// Parallel
	args = append(args, "-parallel", "none")

	// Serial
	serialSock := filepath.Join(cfg.InstanceDir, filenames.SerialSock)
	if err := os.RemoveAll(serialSock); err != nil {
		return "", nil, err
	}
	serialLog := filepath.Join(cfg.InstanceDir, filenames.SerialLog)
	if err := os.RemoveAll(serialLog); err != nil {
		return "", nil, err
	}
	const serialChardev = "char-serial"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off,logfile=%s", serialChardev, serialSock, serialLog))
	args = append(args, "-serial", "chardev:"+serialChardev)

	// We also want to enable vsock here, but QEMU does not support vsock for macOS hosts

	if *y.MountType == limayaml.NINEP {
		for i, f := range y.Mounts {
			tag := fmt.Sprintf("mount%d", i)
			location, err := localpathutil.Expand(f.Location)
			if err != nil {
				return "", nil, err
			}
			if err := os.MkdirAll(location, 0755); err != nil {
				return "", nil, err
			}
			options := "local"
			options += fmt.Sprintf(",mount_tag=%s", tag)
			options += fmt.Sprintf(",path=%s", location)
			options += fmt.Sprintf(",security_model=%s", *f.NineP.SecurityModel)
			if !*f.Writable {
				options += ",readonly"
			}
			args = append(args, "-virtfs", options)
		}
	}

	// QMP
	qmpSock := filepath.Join(cfg.InstanceDir, filenames.QMPSock)
	if err := os.RemoveAll(qmpSock); err != nil {
		return "", nil, err
	}
	const qmpChardev = "char-qmp"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off", qmpChardev, qmpSock))
	args = append(args, "-qmp", "chardev:"+qmpChardev)

	// QEMU process
	args = append(args, "-name", "lima-"+cfg.Name)
	args = append(args, "-pidfile", filepath.Join(cfg.InstanceDir, filenames.QemuPID))

	return exe, args, nil
}

func getExe(arch limayaml.Arch) (string, []string, error) {
	exeBase := "qemu-system-" + arch
	var args []string
	envK := "QEMU_SYSTEM_" + strings.ToUpper(arch)
	if envV := os.Getenv(envK); envV != "" {
		ss, err := shellwords.Parse(envV)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse %s value %q: %w", envK, envV, err)
		}
		exeBase, args = ss[0], ss[1:]
		if len(args) != 0 {
			logrus.Warnf("Specifying args (%v) via $%s is supported only for debugging!", args, envK)
		}
	}
	exe, err := exec.LookPath(exeBase)
	if err != nil {
		return "", nil, err
	}
	return exe, args, nil
}

func getAccel(arch limayaml.Arch) string {
	if limayaml.IsNativeArch(arch) {
		switch runtime.GOOS {
		case "darwin":
			return "hvf"
		case "linux":
			return "kvm"
		case "netbsd":
			return "nvmm"
		case "windows":
			return "whpx"
		}
	}
	return "tcg"
}

func getFirmware(qemuExe string, arch limayaml.Arch) (string, error) {
	switch arch {
	case limayaml.X8664, limayaml.AARCH64:
	default:
		return "", fmt.Errorf("unexpected architecture: %q", arch)
	}

	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	binDir := filepath.Dir(qemuExe)                              // "/usr/local/bin"
	localDir := filepath.Dir(binDir)                             // "/usr/local"
	userLocalDir := filepath.Join(currentUser.HomeDir, ".local") // "$HOME/.local"

	relativePath := fmt.Sprintf("share/qemu/edk2-%s-code.fd", arch)
	candidates := []string{
		filepath.Join(userLocalDir, relativePath), // XDG-like
		filepath.Join(localDir, relativePath),     // macOS (homebrew)
	}

	switch arch {
	case limayaml.X8664:
		// Debian package "ovmf"
		candidates = append(candidates, "/usr/share/OVMF/OVMF_CODE.fd")
		// openSUSE package "qemu-ovmf-x86_64"
		candidates = append(candidates, "/usr/share/qemu/ovmf-x86_64-code.bin")
		// Archlinux package "edk2-ovmf"
		candidates = append(candidates, "/usr/share/edk2-ovmf/x64/OVMF_CODE.fd")
	case limayaml.AARCH64:
		// Debian package "qemu-efi-aarch64"
		candidates = append(candidates, "/usr/share/AAVMF/AAVMF_CODE.fd")
		// Debian package "qemu-efi-aarch64" (unpadded, backwards compatibility)
		candidates = append(candidates, "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd")
	}

	logrus.Debugf("firmware candidates = %v", candidates)

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}

	if arch == limayaml.X8664 {
		return "", fmt.Errorf("could not find firmware for %q (hint: try setting `firmware.legacyBIOS` to `true`)", qemuExe)
	}
	return "", fmt.Errorf("could not find firmware for %q (hint: try copying the \"edk-%s-code.fd\" firmware to $HOME/.local/share/qemu/)", arch, qemuExe)
}
