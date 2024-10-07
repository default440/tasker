package cmd

import (
	"context"
	"os"
	"tasker/browser"
	"tasker/tfs"
	"tasker/tfs/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	createTaskCmd = &cobra.Command{
		Use:   "create <Title> [Description]",
		Short: "Create new task",
		Long:  `Create new task in current sprint`,
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			title := ""
			if len(args) > 0 {
				title = args[0]
			}

			description := ""
			if len(args) > 1 {
				description = args[1]
			}

			err := createTaskCommand(cmd.Context(), title, description)

			cobra.CheckErr(err)
		},
	}

	createTaskCmdFlagEstimate          float32
	createTaskCmdFlagParentWorkItemID  uint32
	createTaskCmdFlagOpenTaskInBrowser bool
	createTaskCmdFlagTags              []string
	createTaskCmdFlagUnassignedTask    bool
	createTaskCmdFlagWorkitemType      string
	createTaskCmdFlagCurrentIteration  bool
)

func init() {
	rootCmd.AddCommand(createTaskCmd)

	createTaskCmd.PersistentFlags().String("project", "", "TFS Project name (NSMS, NVC, etc)")
	createTaskCmd.PersistentFlags().String("team", "", "The team")
	createTaskCmd.PersistentFlags().String("discipline", "", "The discipline to which the task belongs")
	createTaskCmd.PersistentFlags().String("user", "", "The User to assign")

	createTaskCmd.PersistentFlags().Float32VarP(&createTaskCmdFlagEstimate, "estimate", "e", 0, "The original estimate of work required to complete the task (in person hours)")
	createTaskCmd.PersistentFlags().Uint32VarP(&createTaskCmdFlagParentWorkItemID, "parent", "p", 0, "Id of parent User Story (if not specified looks up by according name pattern)")
	createTaskCmd.PersistentFlags().BoolVarP(&createTaskCmdFlagOpenTaskInBrowser, "open", "o", false, "Open created task in browser?")
	createTaskCmd.PersistentFlags().StringSliceVarP(&createTaskCmdFlagTags, "tag", "t", []string{}, "Tags of the task. Can be separated by comma or specified multiple times.")
	createTaskCmd.PersistentFlags().BoolVarP(&createTaskCmdFlagUnassignedTask, "unassigned", "u", false, "Do not assign task")
	createTaskCmd.PersistentFlags().StringVarP(&createTaskCmdFlagWorkitemType, "type", "", "Task", "Type of workitem (Task, Requirement, etc.)")
	createTaskCmd.PersistentFlags().BoolVarP(&createTaskCmdFlagCurrentIteration, "current", "c", false, "Assign current iteration path")

	cobra.CheckErr(createTaskCmd.MarkPersistentFlagRequired("estimate"))

	cobra.CheckErr(viper.BindPFlag("tfsProject", createTaskCmd.PersistentFlags().Lookup("project")))
	cobra.CheckErr(viper.BindPFlag("tfsTeam", createTaskCmd.PersistentFlags().Lookup("team")))
	cobra.CheckErr(viper.BindPFlag("tfsDiscipline", createTaskCmd.PersistentFlags().Lookup("discipline")))
	cobra.CheckErr(viper.BindPFlag("tfsUserFilter", createTaskCmd.PersistentFlags().Lookup("user")))
}

func createTaskCommand(ctx context.Context, title, description string) error {
	spinner, _ := pterm.DefaultSpinner.Start()
	defer func() {
		_ = spinner.Stop()
	}()

	parentUserStoryNamePattern := viper.GetString("tfsCommonUserStoryNamePattern")

	api, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	task, err := api.CreateWorkItem(
		ctx,
		createTaskCmdFlagWorkitemType,
		title,
		description,
		createTaskCmdFlagEstimate,
		int(createTaskCmdFlagParentWorkItemID),
		nil,
		createTaskCmdFlagTags,
		parentUserStoryNamePattern,
		!createTaskCmdFlagUnassignedTask,
		createTaskCmdFlagCurrentIteration,
	)
	printCreateTaskResult(task, err, spinner)
	openInBrowser(task)

	return err
}

func printCreateTaskResult(task *workitemtracking.WorkItem, err error, spinner *pterm.SpinnerPrinter) {
	if task != nil {
		taskURL := workitem.GetURL(task)
		if err == nil {
			spinner.Success(taskURL)
		} else {
			spinner.Warning(taskURL)
		}
	}

	if err != nil {
		spinner.Fail(err.Error())
		os.Exit(1)
	}
}

func openInBrowser(task *workitemtracking.WorkItem) {
	if createTaskCmdFlagOpenTaskInBrowser && task != nil {
		href := workitem.GetURL(task)
		_ = browser.OpenURL(href)
	}
}
