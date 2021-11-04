package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"tasker/tfs"
	"tasker/tfs/workitem"

	"github.com/spf13/cobra"
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
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	bug, err := a.Client.Get(ctx, bugID)
	if err != nil {
		return err
	}

	relations := []*workitem.Relation{
		{
			URL:  *bug.Url,
			Type: "System.LinkTypes.Related",
		},
	}

	title := fmt.Sprintf("BUG %d %s", bugID, workitem.GetTitle(bug))
	return a.CreateTask(ctx, title, description, int(estimate), int(parentWorkItemID), relations, []string{"bugfix"}, openBrowser)
}
