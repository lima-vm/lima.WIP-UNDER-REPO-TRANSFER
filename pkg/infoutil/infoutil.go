package infoutil

import (
	"github.com/lima-vm/lima/pkg/driverutil"
	"github.com/lima-vm/lima/pkg/imagestore"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/lima-vm/lima/pkg/version"
)

type Info struct {
	Version         string                   `json:"version"`
	Templates       []templatestore.Template `json:"templates"`
	DefaultTemplate *limayaml.LimaYAML       `json:"defaultTemplate"`
	Images          []imagestore.Image       `json:"images"`
	DefaultImage    *limayaml.ImageYAML      `json:"defaultImage"`
	LimaHome        string                   `json:"limaHome"`
	VMTypes         []string                 `json:"vmTypes"` // since Lima v0.14.2
}

func GetInfo() (*Info, error) {
	b, err := templatestore.Read(templatestore.Default)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(b, "")
	if err != nil {
		return nil, err
	}
	bi, err := imagestore.Read(imagestore.Default)
	if err != nil {
		return nil, err
	}
	yi, err := limayaml.LoadImage(bi, "")
	if err != nil {
		return nil, err
	}
	info := &Info{
		Version:         version.Version,
		DefaultTemplate: y,
		DefaultImage:    yi,
		VMTypes:         driverutil.Drivers(),
	}
	info.Templates, err = templatestore.Templates()
	if err != nil {
		return nil, err
	}
	info.Images, err = imagestore.Images()
	if err != nil {
		return nil, err
	}
	info.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	return info, nil
}
