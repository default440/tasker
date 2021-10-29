package cmd

import (
	"context"
	"fmt"

	"tasker/api/connection"
	"tasker/api/identity"
	"tasker/api/work"
	"tasker/api/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	createCmd = &cobra.Command{
		Use:   "create TITLE [Description]",
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

	estimate         uint8
	parentWorkItemID uint32
)

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.PersistentFlags().String("project", "", "TFS Project name (NSMS, NVC, etc)")
	createCmd.PersistentFlags().String("team", "", "The team")
	createCmd.PersistentFlags().String("discipline", "", "The discipline to which the task belongs")
	createCmd.PersistentFlags().String("user", "", "The User to assign")

	createCmd.PersistentFlags().Uint8VarP(&estimate, "estimate", "e", 0, "The original estimate of work required to complete the task (in person hours)")
	createCmd.PersistentFlags().Uint32VarP(&parentWorkItemID, "parent", "p", 0, "Id of parent User Story (default to '*Общие задачи*' of current sprint)")

	cobra.CheckErr(createCmd.MarkPersistentFlagRequired("estimate"))

	cobra.CheckErr(viper.BindPFlag("project", createCmd.PersistentFlags().Lookup("project")))
	cobra.CheckErr(viper.BindPFlag("team", createCmd.PersistentFlags().Lookup("team")))
	cobra.CheckErr(viper.BindPFlag("discipline", createCmd.PersistentFlags().Lookup("discipline")))
	cobra.CheckErr(viper.BindPFlag("user", createCmd.PersistentFlags().Lookup("user")))
}

func createTaskCommand(ctx context.Context, title, description string) error {
	conn := connection.Create()
	project := viper.GetString("project")
	team := viper.GetString("team")
	discipline := viper.GetString("discipline")
	user := viper.GetString("user")

	currentIterationPath, err := work.GetCurrentIteration(ctx, conn, project, team)
	if err != nil {
		return err
	}

	api, err := workitem.NewClient(ctx, conn, team, project)
	if err != nil {
		return err
	}

	var parent *workitemtracking.WorkItemReference
	if parentWorkItemID > 0 {
		parent, err = api.GetReference(ctx, int(parentWorkItemID))
	} else {
		parent, err = api.FindCommonUserStory(ctx, currentIterationPath)
	}
	if err != nil {
		return err
	}

	links := []*workitem.Link{
		{
			URL:  *parent.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		},
	}
	task, err := api.Create(ctx, title, description, discipline, currentIterationPath, int(estimate), links, []string{})
	if err != nil {
		return err
	}

	href := workitem.GetURL(task)
	fmt.Printf("%s\n", href)

	userIdentity, err := identity.GetCurrent(ctx, conn, user)
	if err != nil {
		return err
	}

	err = api.Assign(ctx, task, userIdentity)
	if err != nil {
		return err
	}

	return nil
}
