package imagestore

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
)

type Image struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

func Read(name string) ([]byte, error) {
	dir, err := usrlocalsharelima.Dir()
	if err != nil {
		return nil, err
	}
	if name == "default" {
		name = Default
	}
	yamlPath, err := securejoin.SecureJoin(filepath.Join(dir, "images"), name+".yaml")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(yamlPath)
}

const Default = "ubuntu-24.04"

func Images() ([]Image, error) {
	usrlocalsharelimaDir, err := usrlocalsharelima.Dir()
	if err != nil {
		return nil, err
	}
	imagesDir := filepath.Join(usrlocalsharelimaDir, "images")

	var res []Image
	walkDirFn := func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, ".") || !strings.HasSuffix(base, ".yaml") {
			return nil
		}
		x := Image{
			// Name is like "ubuntu-24.04", "debian-12", ...
			Name:     strings.TrimSuffix(strings.TrimPrefix(p, imagesDir+"/"), ".yaml"),
			Location: p,
		}
		res = append(res, x)
		return nil
	}
	if err = filepath.WalkDir(imagesDir, walkDirFn); err != nil {
		return nil, err
	}
	return res, nil
}
