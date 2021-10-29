package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"tasker/api/connection"
	"tasker/api/identity"
	"tasker/api/work"
	"tasker/api/workitem"

	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var bugfixCmd = &cobra.Command{
	Use:   "bugfix BUG_ID [Description]",
	Short: "Create task for bug",
	Long:  "Create task for bug related work.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires a bug ID argument")
		}

		if id, err := strconv.Atoi(args[0]); err != nil || id <= 0 {
			return errors.New("invalid bug ID specified")
		}

		if len(args) > 2 {
			return fmt.Errorf("accepts at most %d arg(s), received %d", 2, len(args))
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		bugID, _ := strconv.Atoi(args[0])
		description := ""
		if len(args) > 1 {
			description = args[1]
		}
		err := createBugfixCommand(cmd.Context(), bugID, description)
		cobra.CheckErr(err)
	},
}

func init() {
	createCmd.AddCommand(bugfixCmd)
}

func createBugfixCommand(ctx context.Context, bugID int, description string) error {
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

	bug, err := api.Get(ctx, bugID)
	if err != nil {
		return err
	}

	links := []*workitem.Link{
		{
			URL:  *parent.Url,
			Type: "System.LinkTypes.Hierarchy-Reverse",
		},
		{
			URL:  *bug.Url,
			Type: "System.LinkTypes.Related",
		},
	}

	title := fmt.Sprintf("BUG %d %s", bugID, workitem.GetTitle(bug))
	task, err := api.Create(ctx, title, description, discipline, currentIterationPath, int(estimate), links, []string{"bugfix"})
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
