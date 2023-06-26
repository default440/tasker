package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"tasker/tfs"
	"tasker/tfs/workitem"
	"text/template"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultBugTitleTemplate = "BugFix {{.ID}} {{.Title}}"

var bugfixCmd = &cobra.Command{
	Use:   "bugfix <Bug ID> [Description]",
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
	createTaskCmd.AddCommand(bugfixCmd)
}

func createBugfixCommand(ctx context.Context, bugID int, description string) error {
	spinner, _ := pterm.DefaultSpinner.Start()
	defer func() {
		_ = spinner.Stop()
	}()

	parentUserStoryNamePattern := viper.GetString("tfsBugfixUserStoryNamePattern")

	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	bug, err := a.WiClient.Get(ctx, bugID)
	if err != nil {
		return err
	}

	relations := []*workitem.Relation{
		{
			URL:  *bug.Url,
			Type: "System.LinkTypes.Related",
		},
	}

	title, err := getBugFixTaskTitle(bug)
	if err != nil {
		return err
	}

	tags = append(tags, "bugfix")
	task, err := a.CreateTask(ctx, title, description, estimate, int(parentWorkItemID), relations, tags, parentUserStoryNamePattern, !unassignedTask)
	printCreateTaskResult(task, err, spinner)
	openInBrowser(task)

	return err
}

func getBugFixTaskTitle(task *workitemtracking.WorkItem) (string, error) {
	type bugTemplateData struct {
		ID    int
		Title string
	}

	templateString := viper.GetString("tfsBugTitleTemplate")
	if templateString == "" {
		templateString = defaultBugTitleTemplate
	}

	t, err := template.New("bug title template").Parse(templateString)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	err = t.Execute(&result, bugTemplateData{
		ID:    *task.Id,
		Title: workitem.GetTitle(task),
	})

	if err != nil {
		return "", err
	}

	return result.String(), nil
}
