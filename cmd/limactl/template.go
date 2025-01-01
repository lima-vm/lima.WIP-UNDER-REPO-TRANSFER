package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/limatmpl"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newTemplateCommand() *cobra.Command {
	templateCommand := &cobra.Command{
		Use:           "template",
		Aliases:       []string{"tmpl"},
		Short:         "Lima template management",
		SilenceUsage:  true,
		SilenceErrors: true,
		GroupID:       advancedCommand,
		// The template command is still hidden because the subcommands and options are still under development
		// and subject to change at any time.
		Hidden: true,
	}
	templateCommand.AddCommand(
		newTemplateCopyCommand(),
		newTemplateValidateCommand(),
	)
	return templateCommand
}

// The validate command exists for backwards compatibility, and because the template command is still hidden.
func newValidateCommand() *cobra.Command {
	validateCommand := newTemplateValidateCommand()
	validateCommand.GroupID = advancedCommand
	return validateCommand
}

var templateCopyExample = `  Template locators are local files, file://, https://, or template:// URLs

  # Copy default template to STDOUT
  limactl template copy template://default -

  # Copy template from web location to local file
  limactl template copy https://example.com/lima.yaml mighty-machine.yaml
`

func newTemplateCopyCommand() *cobra.Command {
	templateCopyCommand := &cobra.Command{
		Use:     "copy [OPTIONS] TEMPLATE DEST",
		Short:   "Copy template",
		Long:    "Copy a template via locator to a local file",
		Example: templateCopyExample,
		Args:    WrapArgsError(cobra.ExactArgs(2)),
		RunE:    templateCopyAction,
	}
	templateCopyCommand.Flags().Bool("embed", false, "embed dependencies into template")
	templateCopyCommand.Flags().Bool("fill", false, "fill defaults")
	templateCopyCommand.Flags().Bool("verbatim", false, "don't make locators absolute")
	return templateCopyCommand
}

func templateCopyAction(cmd *cobra.Command, args []string) error {
	embed, err := cmd.Flags().GetBool("embed")
	if err != nil {
		return err
	}
	fill, err := cmd.Flags().GetBool("fill")
	if err != nil {
		return err
	}
	verbatim, err := cmd.Flags().GetBool("verbatim")
	if err != nil {
		return err
	}
	if embed && verbatim {
		return errors.New("--embed and --verbatim cannot be used together")
	}
	if fill && verbatim {
		return errors.New("--fill and --verbatim cannot be used together")
	}

	tmpl, err := limatmpl.Read(cmd.Context(), "", args[0])
	if err != nil {
		return err
	}
	if len(tmpl.Bytes) == 0 {
		return fmt.Errorf("don't know how to interpret %q as a template locator", args[0])
	}
	if !verbatim {
		if embed {
			if err := tmpl.Embed(cmd.Context()); err != nil {
				return err
			}
		} else {
			if err := tmpl.UseAbsLocators(); err != nil {
				return err
			}
		}
	}
	if fill {
		limaDir, err := dirnames.LimaDir()
		if err != nil {
			return err
		}
		// Load() will merge the template with override.yaml and default.yaml via FillDefaults().
		// FillDefaults() needs the potential instance directory to validate host templates using {{.Dir}}.
		filePath := filepath.Join(limaDir, tmpl.Name+".yaml")
		tmpl.Config, err = limayaml.Load(tmpl.Bytes, filePath)
		if err != nil {
			return err
		}
		tmpl.Bytes, err = limayaml.Marshal(tmpl.Config, false)
		if err != nil {
			return err
		}
	}
	writer := cmd.OutOrStdout()
	target := args[1]
	if target != "-" {
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	_, err = fmt.Fprint(writer, string(tmpl.Bytes))
	return err
}

func newTemplateValidateCommand() *cobra.Command {
	templateValidateCommand := &cobra.Command{
		Use:   "validate TEMPLATE [TEMPLATE, ...]",
		Short: "Validate YAML templates",
		Args:  WrapArgsError(cobra.MinimumNArgs(1)),
		RunE:  templateValidateAction,
	}
	templateValidateCommand.Flags().Bool("fill", false, "fill defaults")
	return templateValidateCommand
}

func templateValidateAction(cmd *cobra.Command, args []string) error {
	fill, err := cmd.Flags().GetBool("fill")
	if err != nil {
		return err
	}
	limaDir, err := dirnames.LimaDir()
	if err != nil {
		return err
	}

	for _, arg := range args {
		tmpl, err := limatmpl.Read(cmd.Context(), "", arg)
		if err != nil {
			return err
		}
		if len(tmpl.Bytes) == 0 {
			return fmt.Errorf("don't know how to interpret %q as a template locator", arg)
		}
		if tmpl.Name == "" {
			return fmt.Errorf("can't determine instance name from template locator %q", arg)
		}
		// Load() will merge the template with override.yaml and default.yaml via FillDefaults().
		// FillDefaults() needs the potential instance directory to validate host templates using {{.Dir}}.
		filePath := filepath.Join(limaDir, tmpl.Name+".yaml")
		y, err := limayaml.Load(tmpl.Bytes, filePath)
		if err != nil {
			return err
		}
		if err := limayaml.Validate(y, false); err != nil {
			return fmt.Errorf("failed to validate YAML file %q: %w", arg, err)
		}
		logrus.Infof("%q: OK", arg)
		if fill {
			b, err := limayaml.Marshal(y, len(args) > 1)
			if err != nil {
				return fmt.Errorf("failed to marshal template %q again after filling defaults: %w", arg, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(b))
		}
	}

	return nil
}
