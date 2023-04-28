package cmd

import (
	"context"
	"fmt"
	"strconv"
	"tasker/wiki"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	wikiCmd = &cobra.Command{
		Use:   "wiki",
		Short: "Manage Wiki pages",
		Long:  `Move wike pages.`,
	}

	moveWikiCmd = &cobra.Command{
		Use:   "move <Page ID, ...> -t <Target ID>",
		Short: "Move wiki pages",
		Long:  `Replace wiki pages under new parent.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pageIDs := make([]uint, len(args))
			for i := 0; i < len(args); i++ {
				pageId, err := strconv.ParseUint(args[i], 10, 32)
				if err != nil {
					cobra.CheckErr(err)
				}
				pageIDs[i] = uint(pageId)
			}
			err := moveWikiPagesCommand(cmd.Context(), pageIDs)
			cobra.CheckErr(err)
		},
	}

	moveWikiCmdFlagNewParentPageID uint
)

func init() {
	rootCmd.AddCommand(wikiCmd)
	wikiCmd.AddCommand(moveWikiCmd)

	moveWikiCmd.Flags().UintVarP(&moveWikiCmdFlagNewParentPageID, "target", "t", 0, "ID of target parent Wiki page")

	cobra.CheckErr(moveWikiCmd.MarkFlagRequired("target"))
}

func moveWikiPagesCommand(ctx context.Context, pageIDs []uint) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(pageIDs)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for i := 0; i < len(pageIDs); i++ {
		progressbar.UpdateTitle(fmt.Sprintf("Moving... %d", pageIDs[i]))
		err = wiki.MovePage(api, pageIDs[i], moveWikiCmdFlagNewParentPageID)
		if err != nil {
			pterm.Error.Println(fmt.Sprintf("NOT MOVED %d: %s", pageIDs[i], err.Error()))
		} else {
			pterm.Success.Println(fmt.Sprintf("MOVED %d", pageIDs[i]))
		}
	}

	_, _ = progressbar.Stop()

	return nil
}
