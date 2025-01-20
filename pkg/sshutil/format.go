package sshutil

import (
	"fmt"
	"io"
	"strings"

	"github.com/lima-vm/lima/pkg/identifierutil"
)

// FormatT specifies the format type.
type FormatT = string

const (
	// FormatCmd prints the full ssh command line.
	//
	//	ssh -o IdentityFile="/Users/example/.lima/_config/user" -o User=example -o Hostname=127.0.0.1 -o Port=60022 lima-default
	FormatCmd = FormatT("cmd")

	// FormatArgs is similar to FormatCmd but omits "ssh" and the destination address.
	//
	//	-o IdentityFile="/Users/example/.lima/_config/user" -o User=example -o Hostname=127.0.0.1 -o Port=60022
	FormatArgs = FormatT("args")

	// FormatOptions prints the ssh option key value pairs.
	//
	//	IdentityFile="/Users/example/.lima/_config/user"
	//	User=example
	//	Hostname=127.0.0.1
	//	Port=60022
	FormatOptions = FormatT("options")

	// FormatConfig uses the ~/.ssh/config format
	//
	//	Host lima-default
	//	  IdentityFile "/Users/example/.lima/_config/user "
	//	  User example
	//	  Hostname 127.0.0.1
	//	  Port 60022
	FormatConfig = FormatT("config")

// TODO: consider supporting "url" format (ssh://USER@HOSTNAME:PORT)
//
// TODO: consider supporting "json" format
// It is unclear whether we can just map ssh "config" into JSON, as "config" has duplicated keys.
// (JSON supports duplicated keys too, but not all JSON implementations expect JSON with duplicated keys)
)

// Formats is the list of the supported formats.
var Formats = []FormatT{FormatCmd, FormatArgs, FormatOptions, FormatConfig}

func quoteOption(o string) string {
	// make sure the shell doesn't swallow quotes in option values
	if strings.ContainsRune(o, '"') {
		o = "'" + o + "'"
	}
	return o
}

// Format formats the ssh options.
func Format(w io.Writer, sshPath, instName string, format FormatT, opts []string) error {
	fakeHostname := identifierutil.HostnameFromInstName(instName) // TODO: support customization
	switch format {
	case FormatCmd:
		args := []string{sshPath}
		for _, o := range opts {
			args = append(args, "-o", quoteOption(o))
		}
		args = append(args, fakeHostname)
		// the args are similar to `limactl shell` but not exactly same. (e.g., lacks -t)
		fmt.Fprintln(w, strings.Join(args, " ")) // no need to use shellescape.QuoteCommand
	case FormatArgs:
		var args []string
		for _, o := range opts {
			args = append(args, "-o", quoteOption(o))
		}
		fmt.Fprintln(w, strings.Join(args, " ")) // no need to use shellescape.QuoteCommand
	case FormatOptions:
		for _, o := range opts {
			fmt.Fprintln(w, o)
		}
	case FormatConfig:
		fmt.Fprintf(w, "Host %s\n", fakeHostname)
		for _, o := range opts {
			kv := strings.SplitN(o, "=", 2)
			if len(kv) != 2 {
				return fmt.Errorf("unexpected option %q", o)
			}
			fmt.Fprintf(w, "  %s %s\n", kv[0], kv[1])
		}
	default:
		return fmt.Errorf("unknown format: %q", format)
	}
	return nil
}
