package cmd

import (
	"context"
	"tasker/tfs"

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
	openBrowser      bool
)

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.PersistentFlags().String("project", "", "TFS Project name (NSMS, NVC, etc)")
	createCmd.PersistentFlags().String("team", "", "The team")
	createCmd.PersistentFlags().String("discipline", "", "The discipline to which the task belongs")
	createCmd.PersistentFlags().String("user", "", "The User to assign")

	createCmd.PersistentFlags().Uint8VarP(&estimate, "estimate", "e", 0, "The original estimate of work required to complete the task (in person hours)")
	createCmd.PersistentFlags().Uint32VarP(&parentWorkItemID, "parent", "p", 0, "Id of parent User Story (default to '*Общие задачи*' of current sprint)")
	createCmd.PersistentFlags().BoolVarP(&openBrowser, "open", "o", false, "Open created task in browser?")

	cobra.CheckErr(createCmd.MarkPersistentFlagRequired("estimate"))

	cobra.CheckErr(viper.BindPFlag("tfsProject", createCmd.PersistentFlags().Lookup("project")))
	cobra.CheckErr(viper.BindPFlag("tfsTeam", createCmd.PersistentFlags().Lookup("team")))
	cobra.CheckErr(viper.BindPFlag("tfsDiscipline", createCmd.PersistentFlags().Lookup("discipline")))
	cobra.CheckErr(viper.BindPFlag("tfsUserFilter", createCmd.PersistentFlags().Lookup("user")))
}

func createTaskCommand(ctx context.Context, title, description string) error {
	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}
	return a.CreateTask(ctx, title, description, int(estimate), int(parentWorkItemID), nil, nil, openBrowser)
}
