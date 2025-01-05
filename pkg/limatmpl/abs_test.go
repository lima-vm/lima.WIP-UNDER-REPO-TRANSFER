package limatmpl

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

type useAbsLocatorsTestCase struct {
	description string
	locator     string
	template    string
	expected    string
}

var useAbsLocatorsTestCases = []useAbsLocatorsTestCase{
	{
		"Template without basedOn or script file",
		"template://foo",
		`foo: bar`,
		`foo: bar`,
	},
	{
		"Single string base template",
		"template://foo",
		`basedOn: bar.yaml`,
		`basedOn: template://bar.yaml`,
	},
	{
		"Flow style array of one base template",
		"template://foo",
		`basedOn: [bar.yaml]`,
		`basedOn: ['template://bar.yaml']`,
	},
	{
		"Block style array of one base template",
		"template://foo",
		`
basedOn:
- bar.yaml
`,
		`
basedOn:
- template://bar.yaml`,
	},
	{
		"Block style of four base templates",
		"template://foo",
		`
basedOn:
- bar.yaml
- template://my
- https://example.com/my.yaml
- baz.yaml
`,
		`
basedOn:
- template://bar.yaml
- template://my
- https://example.com/my.yaml
- template://baz.yaml
`,
	},
	{
		"Provisioning and probe scripts",
		"template://experimental/foo",
		`
provision:
- mode: user
  file: script.sh
probes:
- file: probe.sh
`,
		`
provision:
- mode: user
  file: template://experimental/script.sh
probes:
- file: template://experimental/probe.sh
`,
	},
}

func TestUseAbsLocators(t *testing.T) {
	for _, tc := range useAbsLocatorsTestCases {
		t.Run(tc.description, func(t *testing.T) { RunUseAbsLocatorTest(t, tc) })
	}
}

func RunUseAbsLocatorTest(t *testing.T, tc useAbsLocatorsTestCase) {
	tmpl := &Template{
		Bytes:   []byte(strings.TrimSpace(tc.template)),
		Locator: tc.locator,
	}
	err := tmpl.UseAbsLocators()
	assert.NilError(t, err, tc.description)

	actual := strings.TrimSpace(string(tmpl.Bytes))
	expected := strings.TrimSpace(tc.expected)
	assert.Equal(t, actual, expected, tc.description)
}

func TestBasePath(t *testing.T) {
	actual, err := basePath("/foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "/")

	actual, err = basePath("/foo/bar")
	assert.NilError(t, err)
	assert.Equal(t, actual, "/foo")

	actual, err = basePath("template://foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "template://")

	actual, err = basePath("template://foo/bar")
	assert.NilError(t, err)
	assert.Equal(t, actual, "template://foo")

	actual, err = basePath("http://host/foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "http://host")

	actual, err = basePath("http://host/foo/bar")
	assert.NilError(t, err)
	assert.Equal(t, actual, "http://host/foo")

	actual, err = basePath("file:///foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "file:///")

	actual, err = basePath("file:///foo/bar")
	assert.NilError(t, err)
	assert.Equal(t, actual, "file:///foo")
}

func TestAbsPath(t *testing.T) {
	// If the locator is already an absolute path, it is returned unchanged (no extension appended either)
	actual, err := absPath("/foo", "/root")
	assert.NilError(t, err)
	assert.Equal(t, actual, "/foo")

	actual, err = absPath("template://foo", "/root")
	assert.NilError(t, err)
	assert.Equal(t, actual, "template://foo")

	actual, err = absPath("http://host/foo", "/root")
	assert.NilError(t, err)
	assert.Equal(t, actual, "http://host/foo")

	actual, err = absPath("file:///foo", "/root")
	assert.NilError(t, err)
	assert.Equal(t, actual, "file:///foo")

	// Can't have relative path when reading from STDIN
	_, err = absPath("foo", "-")
	assert.ErrorContains(t, err, "STDIN")

	// Relative paths must be underneath the basePath
	_, err = absPath("../foo", "/root")
	assert.ErrorContains(t, err, "'../'")

	// basePath must not be empty
	_, err = absPath("foo", "")
	assert.ErrorContains(t, err, "empty")

	_, err = absPath("./foo", "")
	assert.ErrorContains(t, err, "empty")

	// Check relative paths with all the supported schemes
	actual, err = absPath("./foo", "/root")
	assert.NilError(t, err)
	assert.Equal(t, actual, "/root/foo")

	actual, err = absPath("foo", "template://")
	assert.NilError(t, err)
	assert.Equal(t, actual, "template://foo")

	actual, err = absPath("bar", "template://foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "template://foo/bar")

	actual, err = absPath("foo", "http://host")
	assert.NilError(t, err)
	assert.Equal(t, actual, "http://host/foo")

	actual, err = absPath("bar", "http://host/foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "http://host/foo/bar")

	actual, err = absPath("foo", "file:///")
	assert.NilError(t, err)
	assert.Equal(t, actual, "file:///foo")

	actual, err = absPath("bar", "file:///foo")
	assert.NilError(t, err)
	assert.Equal(t, actual, "file:///foo/bar")
}
