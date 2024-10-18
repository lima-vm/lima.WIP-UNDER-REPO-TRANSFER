package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/pause"
	"github.com/lima-vm/lima/pkg/store"

	"github.com/spf13/cobra"
)

func newResumeCommand() *cobra.Command {
	resumeCmd := &cobra.Command{
		Use:               "resume INSTANCE",
		Short:             "Resume (unpause) an instance",
		Aliases:           []string{"unpause"},
		Args:              cobra.MaximumNArgs(1),
		RunE:              resumeAction,
		ValidArgsFunction: resumeBashComplete,
	}

	return resumeCmd
}

func resumeAction(cmd *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	if inst.Status != store.StatusPaused {
		return fmt.Errorf("expected status %q, got %q", store.StatusPaused, inst.Status)
	}

	ctx := cmd.Context()
	return pause.Resume(ctx, inst)
}

func resumeBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
