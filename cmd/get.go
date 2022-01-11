package cmd

import (
	"context"
	"strconv"

	"tasker/prettyprint"
	"tasker/tfs"

	"github.com/spf13/cobra"
)

var (
	getWorkItemCmd = &cobra.Command{
		Use:   "get <Work Item ID>",
		Short: "Get work item",
		Long:  "Get work item by ID.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			workItemID, err := strconv.Atoi(args[0])
			cobra.CheckErr(err)

			err = getWorkItemCommand(cmd.Context(), workItemID)
			cobra.CheckErr(err)
		},
	}
)

func init() {
	rootCmd.AddCommand(getWorkItemCmd)
}

func getWorkItemCommand(ctx context.Context, workItemID int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	workItem, err := a.Client.Get(ctx, workItemID)
	if err != nil {
		return err
	}

	prettyprint.JSONObject(workItem)

	return nil
}
