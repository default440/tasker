package cmd

import (
	"context"
	"regexp"

	"tasker/tfs"
	"tasker/tfs/pr"

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

	workItemIDRegexp = regexp.MustCompile(`\-\d{5,}`)

	createPrCmdFlagSquash     bool
	createPrCmdFlagProject    string
	createPrCmdFlagRepository string
)

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.AddCommand(createPrCmd)

	createPrCmd.Flags().BoolVarP(&createPrCmdFlagSquash, "squash", "s", true, "Squash PR")
	createPrCmd.Flags().StringVarP(&createPrCmdFlagProject, "project", "p", "NSMS", "TFS project name")
	createPrCmd.Flags().StringVarP(&createPrCmdFlagRepository, "repository", "r", "", "Repository name (by default from suggestion)")
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
