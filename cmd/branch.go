package cmd

import (
	"context"
	"fmt"

	"tasker/tfs"
	"tasker/tfs/repositories"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	branchCmd = &cobra.Command{
		Use:   "branch",
		Short: "Tasks on git branches",
	}

	listBranchesCmd = &cobra.Command{
		Use:   "list",
		Short: "List branches",
		Long:  "List project branches across all repositories that match filter.",
		Run: func(cmd *cobra.Command, args []string) {
			err := listBranchesCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	listBranchesCmdFlagProject string
	listBranchesCmdFlagFilter  string
	listBranchesCmdFlagDelete  bool
)

func init() {
	rootCmd.AddCommand(branchCmd)
	branchCmd.AddCommand(listBranchesCmd)

	listBranchesCmd.Flags().StringVarP(&listBranchesCmdFlagProject, "project", "p", "", "Project name")
	listBranchesCmd.Flags().StringVarP(&listBranchesCmdFlagFilter, "filter", "f", "", "Branch filter")
	listBranchesCmd.Flags().BoolVarP(&listBranchesCmdFlagDelete, "delete", "d", false, "Delete branches")

	cobra.CheckErr(listBranchesCmd.MarkFlagRequired("filter"))
	cobra.CheckErr(listBranchesCmd.MarkFlagRequired("project"))
}

func listBranchesCommand(ctx context.Context) error {
	spinner, _ := pterm.DefaultSpinner.Start()
	defer func() {
		_ = spinner.Stop()
	}()

	project := listBranchesCmdFlagProject
	filter := listBranchesCmdFlagFilter

	api, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	client, err := repositories.NewClient(ctx, api.Conn)
	if err != nil {
		return err
	}

	reps, err := client.GetBranches(ctx, project, filter)
	if err != nil {
		return err
	}

	if len(reps) == 0 {
		spinner.WithMessageStyle(pterm.NewStyle(pterm.FgCyan)).UpdateText("nothing found")
	} else {
		_ = spinner.Stop()
	}

	for _, rep := range reps {
		fmt.Println(rep.Name)
		for _, b := range rep.Branches {
			fmt.Printf("  %s\n", b.DisplayName())
		}

		if listBranchesCmdFlagDelete {
			err = client.DeleteBranches(ctx, project, rep.Name, rep.Branches)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
