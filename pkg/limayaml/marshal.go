package limayaml

import (
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/lima-vm/lima/pkg/yqutil"
	"github.com/sirupsen/logrus"
)

const (
	documentStart = "---\n"
	documentEnd   = "...\n"
)

// Marshal the struct as a YAML document, optionally as a stream.
func Marshal(y *LimaYAML, stream bool) ([]byte, error) {
	b, err := yaml.Marshal(y)
	if err != nil {
		return nil, err
	}
	if stream {
		doc := documentStart + string(b) + documentEnd
		b = []byte(doc)
	}
	return b, nil
}

func unmarshalDisk(dst *Disk, b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		*dst = Disk{Name: s}
		return nil
	}
	return yaml.Unmarshal(b, dst)
}

// unmarshalBaseTemplates unmarshalls `basedOn` which is either a string or a list of strings.
func unmarshalBaseTemplates(dst *BaseTemplates, b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err == nil {
		*dst = BaseTemplates{s}
		return nil
	}
	return yaml.Unmarshal(b, dst)
}

func Unmarshal(data []byte, v interface{}, comment string) error {
	opts := []yaml.DecodeOption{
		yaml.CustomUnmarshaler[BaseTemplates](unmarshalBaseTemplates),
		yaml.CustomUnmarshaler[Disk](unmarshalDisk),
	}
	if err := yaml.UnmarshalWithOptions(data, v, opts...); err != nil {
		return fmt.Errorf("failed to unmarshal YAML (%s): %w", comment, err)
	}
	// The go-yaml library doesn't catch all markup errors, unfortunately
	// make sure to get a "second opinion", using the same library as "yq"
	if err := yqutil.ValidateContent(data); err != nil {
		return fmt.Errorf("failed to unmarshal YAML (%s): %w", comment, err)
	}
	// Finally log a warning if the YAML file violates the "strict" rules
	opts = append(opts, yaml.Strict())
	if err := yaml.UnmarshalWithOptions(data, v, opts...); err != nil {
		logrus.WithField("comment", comment).WithError(err).Warn("Non-strict YAML is deprecated and will be unsupported in a future version of Lima")
		// Non-strict YAML is known to be used by Rancher Desktop:
		// https://github.com/rancher-sandbox/rancher-desktop/blob/c7ea7508a0191634adf16f4675f64c73198e8d37/src/backend/lima.ts#L114-L117
	}
	return nil
}
