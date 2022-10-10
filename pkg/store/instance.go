package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/docker/go-units"
	hostagentclient "github.com/lima-vm/lima/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type Status = string

const (
	StatusUnknown Status = ""
	StatusBroken  Status = "Broken"
	StatusStopped Status = "Stopped"
	StatusRunning Status = "Running"
)

type Instance struct {
	Name         string             `json:"name"`
	Status       Status             `json:"status"`
	Dir          string             `json:"dir"`
	Arch         limayaml.Arch      `json:"arch"`
	CPUType      string             `json:"cpuType"`
	CPUs         int                `json:"cpus,omitempty"`
	Memory       int64              `json:"memory,omitempty"` // bytes
	Disk         int64              `json:"disk,omitempty"`   // bytes
	Message      string             `json:"message,omitempty"`
	Networks     []limayaml.Network `json:"network,omitempty"`
	SSHLocalPort int                `json:"sshLocalPort,omitempty"`
	HostAgentPID int                `json:"hostAgentPID,omitempty"`
	QemuPID      int                `json:"qemuPID,omitempty"`
	Errors       []error            `json:"errors,omitempty"`
}

func (inst *Instance) LoadYAML() (*limayaml.LimaYAML, error) {
	if inst.Dir == "" {
		return nil, errors.New("inst.Dir is empty")
	}
	yamlPath := filepath.Join(inst.Dir, filenames.LimaYAML)
	return LoadYAMLByFilePath(yamlPath)
}

// Inspect returns err only when the instance does not exist (os.ErrNotExist).
// Other errors are returned as *Instance.Errors
func Inspect(instName string) (*Instance, error) {
	inst := &Instance{
		Name:   instName,
		Status: StatusUnknown,
	}
	// InstanceDir validates the instName but does not check whether the instance exists
	instDir, err := InstanceDir(instName)
	if err != nil {
		return nil, err
	}
	// Make sure inst.Dir is set, even when YAML validation fails
	inst.Dir = instDir
	yamlPath := filepath.Join(instDir, filenames.LimaYAML)
	y, err := LoadYAMLByFilePath(yamlPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		inst.Errors = append(inst.Errors, err)
		return inst, nil
	}
	inst.Arch = *y.Arch
	cpuType, ok := y.CPUType[*y.Arch]
	if !ok || cpuType == nil {
		return nil, fmt.Errorf("no cpu type is defined for arch %q", *y.Arch)
	}
	inst.CPUType = *cpuType

	inst.CPUs = *y.CPUs
	memory, err := units.RAMInBytes(*y.Memory)
	if err == nil {
		inst.Memory = memory
	}
	disk, err := units.RAMInBytes(*y.Disk)
	if err == nil {
		inst.Disk = disk
	}
	inst.Networks = y.Networks
	inst.SSHLocalPort = *y.SSH.LocalPort // maybe 0

	inst.HostAgentPID, err = ReadPIDFile(filepath.Join(instDir, filenames.HostAgentPID))
	if err != nil {
		inst.Status = StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	if inst.HostAgentPID != 0 {
		haSock := filepath.Join(instDir, filenames.HostAgentSock)
		haClient, err := hostagentclient.NewHostAgentClient(haSock)
		if err != nil {
			inst.Status = StatusBroken
			inst.Errors = append(inst.Errors, fmt.Errorf("failed to connect to %q: %w", haSock, err))
		} else {
			ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
			defer cancel()
			info, err := haClient.Info(ctx)
			if err != nil {
				inst.Status = StatusBroken
				inst.Errors = append(inst.Errors, fmt.Errorf("failed to get Info from %q: %w", haSock, err))
			} else {
				inst.SSHLocalPort = info.SSHLocalPort
			}
		}
	}

	inst.QemuPID, err = ReadPIDFile(filepath.Join(instDir, filenames.QemuPID))
	if err != nil {
		inst.Status = StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	if inst.Status == StatusUnknown {
		if inst.HostAgentPID > 0 && inst.QemuPID > 0 {
			inst.Status = StatusRunning
		} else if inst.HostAgentPID == 0 && inst.QemuPID == 0 {
			inst.Status = StatusStopped
		} else if inst.HostAgentPID > 0 && inst.QemuPID == 0 {
			inst.Errors = append(inst.Errors, errors.New("host agent is running but qemu is not"))
			inst.Status = StatusBroken
		} else {
			inst.Errors = append(inst.Errors, errors.New("qemu is running but host agent is not"))
			inst.Status = StatusBroken
		}
	}

	tmpl, err := template.New("format").Parse(y.Message)
	if err != nil {
		inst.Errors = append(inst.Errors, fmt.Errorf("message %q is not a valid template: %w", y.Message, err))
		inst.Status = StatusBroken
	} else {
		data, err := AddGlobalFields(inst)
		if err != nil {
			inst.Errors = append(inst.Errors, fmt.Errorf("cannot add global fields to instance data: %w", err))
			inst.Status = StatusBroken
		} else {
			var message strings.Builder
			err = tmpl.Execute(&message, data)
			if err != nil {
				inst.Errors = append(inst.Errors, fmt.Errorf("cannot execute template %q: %w", y.Message, err))
				inst.Status = StatusBroken
			} else {
				inst.Message = message.String()
			}
		}
	}
	return inst, nil
}

// ReadPIDFile returns 0 if the PID file does not exist or the process has already terminated
// (in which case the PID file will be removed).
func ReadPIDFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = os.Remove(path)
			return 0, nil
		}
		// We may not have permission to send the signal (e.g. to network daemon running as root).
		// But if we get a permissions error, it means the process is still running.
		if !errors.Is(err, os.ErrPermission) {
			return 0, err
		}
	}
	return pid, nil
}

type FormatData struct {
	Instance
	HostOS       string
	HostArch     string
	LimaHome     string
	IdentityFile string
}

func AddGlobalFields(inst *Instance) (FormatData, error) {
	var data FormatData
	data.Instance = *inst
	// Add HostOS
	data.HostOS = runtime.GOOS
	// Add HostArch
	data.HostArch = limayaml.NewArch(runtime.GOARCH)
	// Add IdentityFile
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return FormatData{}, err
	}
	data.IdentityFile = filepath.Join(configDir, filenames.UserPrivateKey)
	// Add LimaHome
	data.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return FormatData{}, err
	}
	return data, nil
}
