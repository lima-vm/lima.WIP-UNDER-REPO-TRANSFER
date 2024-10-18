package main

import (
	"fmt"

	"github.com/lima-vm/lima/pkg/pause"
	"github.com/lima-vm/lima/pkg/store"

	"github.com/spf13/cobra"
)

func newSuspendCommand() *cobra.Command {
	suspendCmd := &cobra.Command{
		Use:               "suspend INSTANCE",
		Short:             "Suspend (pause) an instance",
		Aliases:           []string{"pause"},
		Args:              cobra.MaximumNArgs(1),
		RunE:              suspendAction,
		ValidArgsFunction: suspendBashComplete,
	}

	return suspendCmd
}

func suspendAction(cmd *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(instName)
	if err != nil {
		return err
	}

	if inst.Status != store.StatusRunning {
		return fmt.Errorf("expected status %q, got %q", store.StatusRunning, inst.Status)
	}

	ctx := cmd.Context()
	return pause.Suspend(ctx, inst)
}

func suspendBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
