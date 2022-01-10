package cmd

import (
	"context"
	"fmt"

	"tasker/tfs"
	"tasker/tfs/repositories"

	"github.com/spf13/cobra"
)

var (
	listBranchesCmd = &cobra.Command{
		Use:   "list-branches",
		Short: "List all branches",
		Long:  "List all branches of current user across all repositories.",
		Run: func(cmd *cobra.Command, args []string) {
			err := listBranchesCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	listBranchesCmdFlagProject string
	listBranchesCmdFlagFilter  string
)

func init() {
	rootCmd.AddCommand(listBranchesCmd)

	listBranchesCmd.Flags().StringVarP(&listBranchesCmdFlagProject, "project", "p", "NSMS", "Project name")
	listBranchesCmd.Flags().StringVarP(&listBranchesCmdFlagFilter, "filter", "f", "", "Branch filter")

	// cobra.CheckErr(listBranchesCmd.MarkFlagRequired("project"))
}

func listBranchesCommand(ctx context.Context) error {
	api, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	client, err := repositories.NewClient(ctx, api.Conn)
	if err != nil {
		return err
	}

	reps, err := client.GetBranches(ctx, listBranchesCmdFlagProject, listBranchesCmdFlagFilter)
	if err != nil {
		return err
	}

	for _, rep := range reps {
		fmt.Println(rep.Name)
		for _, b := range rep.Branches {
			fmt.Printf("  %s\n", b)
		}
	}

	return nil
}
