package cmd

import (
	"context"
	"fmt"
	"slices"
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
	getWorkItemsCmd = &cobra.Command{
		Use:   "get <Work Item ID, ...>",
		Short: "Get work items",
		Long:  "Get work items by ID.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var workItemIDs []int

			for i := range args {
				workItemID, err := strconv.Atoi(args[i])
				cobra.CheckErr(err)
				workItemIDs = append(workItemIDs, workItemID)
			}

			err := getWorkItemsCommand(cmd.Context(), workItemIDs)
			cobra.CheckErr(err)
		},
	}

	deleteWorkItemsCmd = &cobra.Command{
		Use:   "delete <Work Item ID, ...>",
		Short: "Delete work items",
		Long:  "Delete work items by ID.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var workItemIDs []int

			for i := range args {
				workItemID, err := strconv.Atoi(args[i])
				cobra.CheckErr(err)
				workItemIDs = append(workItemIDs, workItemID)
			}

			err := deleteWorkItemsCommand(cmd.Context(), workItemIDs)
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

			err = copyWorkItemsCommand(cmd.Context(), workItemID)
			cobra.CheckErr(err)
		},
	}

	queryWorkItemsCmd = &cobra.Command{
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

	changeWorkItemsParentCmd = &cobra.Command{
		Use:   "change-parent <Work Item ID, ...>",
		Short: "Changing parent of work items",
		Long:  "Changing parent of work items by ID.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var workItemIDs []int

			for i := range args {
				workItemID, err := strconv.Atoi(args[i])
				cobra.CheckErr(err)
				workItemIDs = append(workItemIDs, workItemID)
			}

			err := changeWorkItemsParentCommand(cmd.Context(), workItemIDs)
			cobra.CheckErr(err)
		},
	}

	copyWorkItemCmdParentID      int
	copyWorkItemCmdIterationPath string
	copyWorkItemCmdAreaPath      string

	queryWorkItemsCmdFlagParent string
	queryWorkItemsCmdFlagType   string
	queryWorkItemsCmdFlagStates []string
	queryWorkItemsCmdFlagActive bool
	queryWorkItemsCmdFlagTags   []string

	changeWorkItemsParentCmdParentID int
)

func init() {
	rootCmd.AddCommand(getWorkItemsCmd)
	rootCmd.AddCommand(deleteWorkItemsCmd)
	rootCmd.AddCommand(copyWorkItemCmd)
	rootCmd.AddCommand(queryWorkItemsCmd)
	rootCmd.AddCommand(closeWorkItemsCmd)
	rootCmd.AddCommand(changeWorkItemsParentCmd)

	copyWorkItemCmd.Flags().IntVarP(&copyWorkItemCmdParentID, "parent", "p", 0, "Id of parent of new Work Item (if specified, then source added as AffectedBy)")
	copyWorkItemCmd.Flags().StringVarP(&copyWorkItemCmdIterationPath, "iteration", "i", "", "Iteration Path of new Work Item")
	copyWorkItemCmd.Flags().StringVarP(&copyWorkItemCmdIterationPath, "area", "a", "", "Area Path of new Work Item")

	queryWorkItemsCmd.Flags().StringVarP(&queryWorkItemsCmdFlagParent, "parent", "p", "", "Work items child of specified work item")
	queryWorkItemsCmd.Flags().StringVarP(&queryWorkItemsCmdFlagType, "type", "t", "", "Work items specified type")
	queryWorkItemsCmd.Flags().StringSliceVarP(&queryWorkItemsCmdFlagStates, "state", "s", nil, "Work items in specified states")
	queryWorkItemsCmd.Flags().BoolVarP(&queryWorkItemsCmdFlagActive, "active", "a", false, "Work items in active state")
	queryWorkItemsCmd.Flags().StringSliceVarP(&queryWorkItemsCmdFlagTags, "tag", "", nil, "Work items tag")

	changeWorkItemsParentCmd.Flags().IntVarP(&changeWorkItemsParentCmdParentID, "parent", "p", 0, "ID of new parent work item")
	cobra.CheckErr(changeWorkItemsParentCmd.MarkFlagRequired("parent"))
}

func changeWorkItemsParentCommand(ctx context.Context, workItemIDs []int) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(workItemIDs)).WithRemoveWhenDone().Start()
	if err == nil {
		defer func() {
			_, _ = progressbar.Stop()
		}()
	} else {
		progressbar = nil
	}

	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	newParentWorkItem, err := a.WiClient.Get(ctx, changeWorkItemsParentCmdParentID)
	if err != nil {
		return err
	}

	workItems, err := a.WiClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &workItemIDs,
		Project: &a.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})
	if err != nil {
		return err
	}

	for _, workItem := range *workItems {
		if progressbar != nil {
			progressbar.UpdateTitle(fmt.Sprintf("Processing %s", workitem.GetTitle(&workItem)))
		}

		relationIndex := slices.IndexFunc(*workItem.Relations, func(rel workitemtracking.WorkItemRelation) bool {
			return *rel.Rel == "System.LinkTypes.Hierarchy-Reverse"
		})
		if relationIndex == -1 {
			relationIndex = 0
		}

		fields := []webapi.JsonPatchOperation{
			{
				Op:   &webapi.OperationValues.Remove,
				Path: ptr.FromStr(fmt.Sprintf("/relations/%d", relationIndex)),
			},
			{
				Op:   &webapi.OperationValues.Add,
				Path: ptr.FromStr("/relations/-"),
				Value: workitemtracking.WorkItemRelation{
					Rel: ptr.FromStr("System.LinkTypes.Hierarchy-Reverse"),
					Url: newParentWorkItem.Url,
				},
			},
		}

		_, err := a.WiClient.UpdateWorkItem(ctx, workitemtracking.UpdateWorkItemArgs{
			Id:       workItem.Id,
			Project:  &a.Project,
			Document: &fields,
		})

		if err != nil {
			return err
		}

		if progressbar != nil {
			progressbar.Increment()
		}
	}

	return nil
}

func closeWorkItemsCommand(ctx context.Context, workItemIDs []int) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(workItemIDs)).WithRemoveWhenDone().Start()
	if err == nil {
		defer func() {
			_, _ = progressbar.Stop()
		}()
	} else {
		progressbar = nil
	}

	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	for _, workItemID := range workItemIDs {
		if progressbar != nil {
			progressbar.UpdateTitle(fmt.Sprintf("Processing %d", workItemID))
		}

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

		if progressbar != nil {
			progressbar.Increment()
		}
	}

	return nil
}

func copyWorkItemsCommand(ctx context.Context, sourceWorkItemID int) error {
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

func getWorkItemsCommand(ctx context.Context, workItemIDs []int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	workItems, err := a.WiClient.GetListExpanded(ctx, workItemIDs)
	if err != nil {
		return err
	}

	prettyprint.JSONObject(workItems)

	return nil
}

func queryWorkItemCommand(ctx context.Context, titlePattern string) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	var workItems []int

	if queryWorkItemsCmdFlagParent != "" {
		childsWiql := `
			SELECT
				[System.Id]
			FROM WorkItemLinks
			WHERE 
				([System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse')
				AND ([Target].[System.Id] = ` + queryWorkItemsCmdFlagParent + `)
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

	if queryWorkItemsCmdFlagActive {
		filters = append(filters, "[State] = 'Active'")
	}

	if queryWorkItemsCmdFlagType != "" {
		filters = append(filters, "[Work Item Type] = '"+queryWorkItemsCmdFlagType+"'")
	}

	var statesFilters []string
	for _, state := range queryWorkItemsCmdFlagStates {
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

func deleteWorkItemsCommand(ctx context.Context, workItemIDs []int) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(workItemIDs)).WithRemoveWhenDone().Start()
	if err == nil {
		defer func() {
			_, _ = progressbar.Stop()
		}()
	} else {
		progressbar = nil
	}

	for _, workItemID := range workItemIDs {
		if progressbar != nil {
			progressbar.UpdateTitle(fmt.Sprintf("Processing %d", workItemID))
		}

		err := a.WiClient.Delete(ctx, workItemID)
		if err != nil {
			return err
		}

		if progressbar != nil {
			progressbar.Increment()
		}
	}

	return nil
}
