package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"tasker/prettyprint"
	"tasker/ptr"
	"tasker/tfs"
	"tasker/tfs/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/pterm/pterm"
	"github.com/samber/lo"
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

	queryWorkItemCmd = &cobra.Command{
		Use:   "query [title pattern]",
		Short: "Query work items",
		Long:  "Query work items by WIQL query.",
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			titlePattern := ""
			if len(args) > 0 {
				titlePattern = args[0]
			}
			err := queryWorkItemCommand(cmd.Context(), titlePattern)
			cobra.CheckErr(err)
		},
	}

	closeWorkItemsCmd = &cobra.Command{
		Use:   "close <Work Item ID, ...>",
		Short: "Close work items",
		Long:  "Close work items by ID.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var workItemIDs []int

			for i := range args {
				workItemID, err := strconv.Atoi(args[i])
				cobra.CheckErr(err)
				workItemIDs = append(workItemIDs, workItemID)
			}

			err := closeWorkItemsCommand(cmd.Context(), workItemIDs)
			cobra.CheckErr(err)
		},
	}

	copyWorkItemCmdParentID      int
	copyWorkItemCmdIterationPath string
	copyWorkItemCmdAreaPath      string

	queryWorkItemCmdFlagParent string
	queryWorkItemCmdFlagType   string
	queryWorkItemCmdFlagStates []string
	queryWorkItemCmdFlagActive bool
	queryWorkItemCmdFlagTags   []string
)

func init() {
	rootCmd.AddCommand(getWorkItemCmd)
	rootCmd.AddCommand(deleteWorkItemCmd)
	rootCmd.AddCommand(copyWorkItemCmd)
	rootCmd.AddCommand(queryWorkItemCmd)
	rootCmd.AddCommand(closeWorkItemsCmd)

	copyWorkItemCmd.Flags().IntVarP(&copyWorkItemCmdParentID, "parent", "p", 0, "Id of parent of new Work Item (if specified, then source added as AffectedBy)")
	copyWorkItemCmd.Flags().StringVarP(&copyWorkItemCmdIterationPath, "iteration", "i", "", "Iteration Path of new Work Item")
	copyWorkItemCmd.Flags().StringVarP(&copyWorkItemCmdIterationPath, "area", "a", "", "Area Path of new Work Item")

	queryWorkItemCmd.Flags().StringVarP(&queryWorkItemCmdFlagParent, "parent", "p", "", "Work items child of specified work item")
	queryWorkItemCmd.Flags().StringVarP(&queryWorkItemCmdFlagType, "type", "t", "", "Work items specified type")
	queryWorkItemCmd.Flags().StringSliceVarP(&queryWorkItemCmdFlagStates, "state", "s", nil, "Work items in specified states")
	queryWorkItemCmd.Flags().BoolVarP(&queryWorkItemCmdFlagActive, "active", "a", false, "Work items in active state")
	queryWorkItemCmd.Flags().StringSliceVarP(&queryWorkItemCmdFlagTags, "tag", "", nil, "Work items tag")
}

func closeWorkItemsCommand(ctx context.Context, workItemIDs []int) error {
	spinner, _ := pterm.DefaultSpinner.Start()
	defer func() {
		_ = spinner.Stop()
	}()

	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	for _, workItemID := range workItemIDs {
		fields := []webapi.JsonPatchOperation{
			{
				Op:    &webapi.OperationValues.Replace,
				Path:  ptr.FromStr("/fields/System.State"),
				Value: "Closed",
			},
		}

		_, err := a.WiClient.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
			Id:       ptr.FromInt(workItemID),
			Project:  &a.Project,
			Document: &fields,
		})

		if err != nil {
			return err
		}
	}

	return nil
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

func queryWorkItemCommand(ctx context.Context, titlePattern string) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	var workItems []int

	if queryWorkItemCmdFlagParent != "" {
		childsWiql := `
			SELECT
				[System.Id]
			FROM WorkItemLinks
			WHERE 
				([System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse')
				AND ([Target].[System.Id] = ` + queryWorkItemCmdFlagParent + `)
		`

		childsResult, err := a.WiClient.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
			Wiql: &workitemtracking.Wiql{
				Query: ptr.FromStr(childsWiql),
			},
			Project: &a.Project,
			Team:    &a.Team,
		})

		if err != nil {
			return err
		}

		workItems = lo.Map(*childsResult.WorkItemRelations, func(wi workitemtracking.WorkItemLink, _ int) int {
			return *wi.Target.Id
		})

		if len(workItems) == 0 {
			return nil
		}
	}

	var filters []string
	if titlePattern != "" {
		filters = append(filters, "[Title] CONTAINS '"+titlePattern+"'")
	}

	if queryWorkItemCmdFlagActive {
		filters = append(filters, "[State] = 'Active'")
	}

	if queryWorkItemCmdFlagType != "" {
		filters = append(filters, "[Work Item Type] = '"+queryWorkItemCmdFlagType+"'")
	}

	var statesFilters []string
	for _, state := range queryWorkItemCmdFlagStates {
		statesFilters = append(statesFilters, "[State] = '"+state+"'")
	}
	if len(statesFilters) > 0 {
		filters = append(filters, "("+strings.Join(statesFilters, " AND ")+")")
	}

	if len(workItems) > 0 {
		filters = append(filters, "[Id] IN ("+strings.Join(lo.Map(workItems, func(id int, _ int) string { return strconv.Itoa(id) }), ",")+")")
	}

	if len(filters) > 0 {
		wiql := "SELECT [Id] FROM WorkItems WHERE " + strings.Join(filters, " AND ")

		result, err := a.WiClient.QueryByWiql(ctx, workitemtracking.QueryByWiqlArgs{
			Wiql: &workitemtracking.Wiql{
				Query: ptr.FromStr(wiql),
			},
			Project: &a.Project,
			Team:    &a.Team,
		})

		if err != nil {
			return err
		}

		workItems = lo.Map(*result.WorkItems, func(wi workitemtracking.WorkItemReference, _ int) int {
			return *wi.Id
		})
	}

	for _, id := range workItems {
		fmt.Printf("%v\n", id)
	}

	return nil
}

func deleteWorkItemCommand(ctx context.Context, workItemID int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	return a.WiClient.Delete(ctx, workItemID)
}
