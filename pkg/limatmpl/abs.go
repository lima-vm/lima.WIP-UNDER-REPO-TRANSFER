package limatmpl

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// UseAbsLocators will replace all relative template locators with absolute ones, so this template
// can be stored anywhere and still reference the same base templates and files.
func (tmpl *Template) UseAbsLocators() error {
	err := tmpl.useAbsLocators()
	return tmpl.ClearOnError(err)
}

func (tmpl *Template) useAbsLocators() error {
	if err := tmpl.Unmarshal(); err != nil {
		return err
	}
	basePath, err := basePath(tmpl.Locator)
	if err != nil {
		return err
	}
	for i, baseLocator := range tmpl.Config.BasedOn {
		locator, err := absPath(baseLocator, basePath)
		if err != nil {
			return err
		}
		if i == 0 {
			// basedOn can either be a single string, or a list of strings
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.basedOn | select(type == \"!!str\")) |= %q\n", locator))
			tmpl.expr.WriteString(fmt.Sprintf("| ($a.basedOn | select(type == \"!!seq\") | .[0]) |= %q\n", locator))
		} else {
			tmpl.expr.WriteString(fmt.Sprintf("| $a.basedOn[%d] = %q\n", i, locator))
		}
	}
	for i, p := range tmpl.Config.Probes {
		if p.File != nil {
			locator, err := absPath(*p.File, basePath)
			if err != nil {
				return err
			}
			tmpl.expr.WriteString(fmt.Sprintf("| $a.probes[%d].file = %q\n", i, locator))
		}
	}
	for i, p := range tmpl.Config.Provision {
		if p.File != nil {
			locator, err := absPath(*p.File, basePath)
			if err != nil {
				return err
			}
			tmpl.expr.WriteString(fmt.Sprintf("| $a.provision[%d].file = %q\n", i, locator))
		}
	}
	return tmpl.evalExpr()
}

// basePath returns the locator without the filename part.
func basePath(locator string) (string, error) {
	u, err := url.Parse(locator)
	if err != nil || u.Scheme == "" {
		return filepath.Abs(filepath.Dir(locator))
	}
	// filepath.Dir("") returns ".", which must be removed for url.JoinPath() to do the right thing later
	return u.Scheme + "://" + strings.TrimSuffix(filepath.Dir(filepath.Join(u.Host, u.Path)), "."), nil
}

// absPath either returns the locator directly, or combines it with the basePath if the locator is a relative path.
func absPath(locator, basePath string) (string, error) {
	u, err := url.Parse(locator)
	if (err == nil && u.Scheme != "") || filepath.IsAbs(locator) {
		return locator, nil
	}
	switch {
	case basePath == "":
		return "", errors.New("basePath is empty")
	case basePath == "-":
		return "", errors.New("can't use relative paths when reading template from STDIN")
	case strings.Contains(locator, "../"):
		return "", fmt.Errorf("relative locator path %q must not contain '../' segments", locator)
	}
	u, err = url.Parse(basePath)
	if err != nil {
		return "", err
	}
	return u.JoinPath(locator).String(), nil
}
