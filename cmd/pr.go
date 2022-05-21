package cmd

import (
	"context"
	"regexp"
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
		Use:   "create",
		Short: "Create PR",
		Long:  "Create pull request assuming best defaults.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			err := createPrCommand(cmd.Context())
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

	workItemIDRegexp = regexp.MustCompile(`\-\d{5,}`)

	createPrCmdFlagSquash     bool
	createPrCmdFlagProject    string
	createPrCmdFlagRepository string
)

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.AddCommand(createPrCmd)
	prCmd.AddCommand(getPrCmd)

	createPrCmd.Flags().BoolVarP(&createPrCmdFlagSquash, "squash", "s", true, "Squash PR")
	createPrCmd.Flags().StringVarP(&createPrCmdFlagProject, "project", "p", "NSMS", "TFS project name")
	createPrCmd.Flags().StringVarP(&createPrCmdFlagRepository, "repository", "r", "", "Repository name (by default from suggestion)")
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

func createPrCommand(ctx context.Context) error {
	spinner, _ := pterm.DefaultSpinner.Start()
	defer func() {
		_ = spinner.Stop()
	}()

	tfsAPI, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	client, err := pr.NewClient(ctx, tfsAPI.Conn, createPrCmdFlagProject)
	if err != nil {
		return err
	}

	var repository string
	if createPrCmdFlagRepository != "" {
		repository = createPrCmdFlagRepository
	} else {
		repository, err = client.FindRepository(ctx)
		if err != nil {
			return err
		}
	}

	creator, err := client.NewCreator(ctx, repository)
	if err != nil {
		return err
	}

	pullrequest, err := creator.Create(ctx, createPrCmdFlagSquash)

	if pullrequest != nil {
		url := pr.GetPullRequestUrl(pullrequest)
		if err == nil {
			spinner.Success(url)
		} else {
			spinner.Warning(url)
		}
	}

	return err
}
