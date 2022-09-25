package cmd

import (
	"context"
	"strconv"

	"tasker/prettyprint"
	"tasker/ptr"
	"tasker/tfs"
	"tasker/tfs/pr"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	prCmd = &cobra.Command{
		Use:   "pr",
		Short: "Manage PR",
		Long:  "View, create etc. pull requests.",
	}

	createPrCmd = &cobra.Command{
		Use:   "create [Merge Message]",
		Short: "Create PR",
		Long:  "Create pull request assuming best defaults.",
		Args:  cobra.RangeArgs(0, 1),
		Run: func(cmd *cobra.Command, args []string) {
			var message string
			if len(args) > 0 {
				message = args[0]
			}

			err := createPrCommand(cmd.Context(), message)
			cobra.CheckErr(err)
		},
	}
	getPrCmd = &cobra.Command{
		Use:   "get <PR ID>",
		Short: "Get pull request",
		Long:  "Get pull request by ID.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			prID, err := strconv.Atoi(args[0])
			cobra.CheckErr(err)

			err = getPrCommand(cmd.Context(), prID)
			cobra.CheckErr(err)
		},
	}

	createPrCmdFlagProject    string
	createPrCmdFlagMessage    string
	createPrCmdFlagRepository string
)

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.AddCommand(createPrCmd)
	prCmd.AddCommand(getPrCmd)

	createPrCmd.Flags().StringVarP(&createPrCmdFlagProject, "project", "p", "NSMS", "TFS project name")
	createPrCmd.Flags().StringVarP(&createPrCmdFlagMessage, "message", "m", "", "Megre commit message")
	createPrCmd.Flags().StringVarP(&createPrCmdFlagRepository, "repository", "r", "", "TFS repository name")
}

func getPrCommand(ctx context.Context, id int) error {
	tfsAPI, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	client, err := pr.NewClient(ctx, tfsAPI.Conn, createPrCmdFlagProject)
	if err != nil {
		return err
	}

	pr, err := client.GetPullRequestById(ctx, git.GetPullRequestByIdArgs{
		PullRequestId: ptr.FromInt(id),
	})
	if err != nil {
		return err
	}

	prettyprint.JSONObject(pr)

	return nil
}

func createPrCommand(ctx context.Context, message string) error {
	tfsAPI, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	client, err := pr.NewClient(ctx, tfsAPI.Conn, createPrCmdFlagProject)
	if err != nil {
		return err
	}

	repository := createPrCmdFlagRepository
	if repository == "" {
		repository, err = client.RequestRepository(ctx)
		if err != nil {
			return err
		}
	}

	creator, err := client.NewCreator(ctx, repository)
	if err != nil {
		return err
	}

	if message == "" {
		message = createPrCmdFlagMessage
	}

	pullrequest, err := creator.Create(ctx, message)

	if pullrequest != nil {
		url := pr.GetPullRequestURL(pullrequest)
		if err == nil {
			pterm.Success.Println(url)
		} else {
			pterm.Warning.Println(url)
		}
	}

	return err
}
