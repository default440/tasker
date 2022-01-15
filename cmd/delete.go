package cmd

import (
	"context"
	"strconv"

	"tasker/tfs"

	"github.com/spf13/cobra"
)

var (
	deleteWorkItemCmd = &cobra.Command{
		Use:   "delete <Work Item ID>",
		Short: "Delete work item",
		Long:  "Delete work item by ID.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			workItemID, err := strconv.Atoi(args[0])
			cobra.CheckErr(err)

			err = deleteWorkItemCommand(cmd.Context(), workItemID)
			cobra.CheckErr(err)
		},
	}
)

func init() {
	rootCmd.AddCommand(deleteWorkItemCmd)
}

func deleteWorkItemCommand(ctx context.Context, workItemID int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	return a.Client.Delete(ctx, workItemID)
}
