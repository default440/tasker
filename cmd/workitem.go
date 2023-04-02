package cmd

import (
	"context"
	"strconv"

	"tasker/prettyprint"
	"tasker/tfs"
	"tasker/tfs/workitem"

	"github.com/pterm/pterm"
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

	copyWorkItemCmd = &cobra.Command{
		Use:   "copy <Work Item ID>",
		Short: "Copy work item",
		Long:  "Copy work item by ID.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			workItemID, err := strconv.Atoi(args[0])
			cobra.CheckErr(err)

			err = copyWorkItemCommand(cmd.Context(), workItemID)
			cobra.CheckErr(err)
		},
	}

	copyWorkItemCmdParentID      int
	copyWorkItemCmdIterationPath string
	copyWorkItemCmdAreaPath      string
)

func init() {
	rootCmd.AddCommand(getWorkItemCmd)
	rootCmd.AddCommand(deleteWorkItemCmd)
	rootCmd.AddCommand(copyWorkItemCmd)

	copyWorkItemCmd.Flags().IntVarP(&copyWorkItemCmdParentID, "parent", "p", 0, "Id of parent of new Work Item (if specified, then source adeded as AffectedBy)")
	copyWorkItemCmd.Flags().StringVarP(&copyWorkItemCmdIterationPath, "iteration", "i", "", "Iteration Path of new Work Item")
	copyWorkItemCmd.Flags().StringVarP(&copyWorkItemCmdIterationPath, "area", "a", "", "Area Path of new Work Item")
}
func copyWorkItemCommand(ctx context.Context, sourceWorkItemID int) error {
	spinner, _ := pterm.DefaultSpinner.Start()
	defer func() {
		_ = spinner.Stop()
	}()

	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	sourceWorkItem, err := a.WiClient.Get(ctx, sourceWorkItemID)
	if err != nil {
		return err
	}

	areaPath := copyWorkItemCmdAreaPath
	if areaPath == "" {
		areaPath = workitem.GetAreaPath(sourceWorkItem)
	}

	iterationPath := copyWorkItemCmdIterationPath
	if iterationPath == "" {
		iterationPath = workitem.GetIterationPath(sourceWorkItem)
	}

	var relations []*workitem.Relation

	if copyWorkItemCmdParentID != 0 {
		parent, err := a.WiClient.Get(ctx, copyWorkItemCmdParentID)
		if err != nil {
			return err
		}

		relations = append(relations, &workitem.Relation{
			URL:  *parent.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		})
		relations = append(relations, &workitem.Relation{
			URL:  *sourceWorkItem.Url,
			Type: "Microsoft.VSTS.Common.Affects-Reverse",
		})
	} else {
		relations = append(relations, &workitem.Relation{
			URL:  *sourceWorkItem.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		})
	}

	tags := workitem.GetTags(sourceWorkItem)
	task, err := a.WiClient.Copy(ctx, sourceWorkItem, areaPath, iterationPath, relations, tags)
	printCreateTaskResult(task, err, spinner)
	openInBrowser(task)

	return err
}

func getWorkItemCommand(ctx context.Context, workItemID int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	workItem, err := a.WiClient.GetExpanded(ctx, workItemID)
	if err != nil {
		return err
	}

	prettyprint.JSONObject(workItem)

	return nil
}

func deleteWorkItemCommand(ctx context.Context, workItemID int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	return a.WiClient.Delete(ctx, workItemID)
}
