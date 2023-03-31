package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"tasker/tfs"
	"tasker/wiki"

	"github.com/eiannone/keyboard"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	goconfluence "github.com/virtomize/confluence-go-api"
	"golang.org/x/sync/errgroup"
)

var (
	techCmd = &cobra.Command{
		Use:   "tech",
		Short: "Manage Tech Debt",
		Long:  `View, create, synchronize etc. techical depbt tasks.`,
	}

	syncTechCmd = &cobra.Command{
		Use:   "sync",
		Short: "Sync tech depbt tasks",
		Long:  `Synchronize techical debt tasks between TFS and Wiki.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := syncTechCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	archiveTechCmd = &cobra.Command{
		Use:   "archive",
		Short: "Archive completed tech debt tasks",
		Long:  `Move completed techical debt tasks under Archive page.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := archiveTechCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	syncTechCmdFlagTfsRequirementID    uint
	syncTechCmdFlagWikiParentPageID    uint
	syncTechCmdFlagWikiTechDebtPageIDs []uint

	archiveTechCmdFlagWikiParentPageID  uint
	archiveTechCmdFlagWikiArchivePageID uint
)

func init() {
	rootCmd.AddCommand(techCmd)
	techCmd.AddCommand(syncTechCmd)
	techCmd.AddCommand(archiveTechCmd)

	syncTechCmd.Flags().UintVarP(&syncTechCmdFlagTfsRequirementID, "requirement", "r", 0, "ID of TFS requirement work item for Tech Debt tasks")
	syncTechCmd.Flags().UintVarP(&syncTechCmdFlagWikiParentPageID, "parent-page", "p", 0, "ID of Wiki parent page with Tech Debt tasks")
	syncTechCmd.Flags().UintSliceVarP(&syncTechCmdFlagWikiTechDebtPageIDs, "debt-page", "d", []uint{}, "ID of Wiki page with Tech Debt task")

	cobra.CheckErr(syncTechCmd.MarkFlagRequired("requirement"))

	archiveTechCmd.Flags().UintVarP(&archiveTechCmdFlagWikiParentPageID, "parent-page", "p", 0, "ID of Wiki parent page with Tech Debt tasks")
	archiveTechCmd.Flags().UintVarP(&archiveTechCmdFlagWikiArchivePageID, "archive-page", "a", 0, "ID of Wiki archive page with completed Tech Debt tasks")

	cobra.CheckErr(archiveTechCmd.MarkFlagRequired("parent-page"))
	cobra.CheckErr(archiveTechCmd.MarkFlagRequired("archive-page"))
}

type techDebtPage struct {
	*wiki.TechDebt
	content *goconfluence.Content
}

func syncTechCommand(ctx context.Context) error {
	wikiParentID := int(syncTechCmdFlagWikiParentPageID)
	requirementID := int(syncTechCmdFlagTfsRequirementID)

	wikiApi, err := wiki.NewClient()
	if err != nil {
		return err
	}

	var pageIDs []string
	if wikiParentID != 0 {
		searchResult, err := wikiApi.GetChildPages(strconv.Itoa(wikiParentID))
		if err != nil {
			return err
		}

		for _, page := range searchResult.Results {
			pageIDs = append(pageIDs, page.ID)
		}
	} else {
		for _, pageID := range syncTechCmdFlagWikiTechDebtPageIDs {
			pageIDs = append(pageIDs, strconv.Itoa(int(pageID)))
		}
	}

	pages, err := parseTechDebtPages(ctx, pageIDs, wikiApi)
	if err != nil {
		return err
	}

	pages = lo.Filter(pages, func(item *techDebtPage, index int) bool {
		return len(item.TfsTasks) == 0 && !item.IsEmptyPage
	})

	if len(pages) == 0 {
		fmt.Println("nothing to create or update")
		return nil
	}

	tfsApi, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	requirement, err := tfsApi.WiClient.Get(ctx, requirementID)
	if err != nil {
		return err
	}

	previewTechDebtTasks(pages)

	ok, err := requestConfirmationTechDebt()
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("canceled by user")
	}

	err = createTechDebtTasks(ctx, pages, tfsApi, wikiApi, requirement)
	if err != nil {
		return err
	}

	return nil
}

func createTechDebtTasks(ctx context.Context, pages []*techDebtPage, tfsApi *tfs.API, wikiApi *goconfluence.API, requirement *workitemtracking.WorkItem) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(pages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, page := range pages {
		progressbar.UpdateTitle(fmt.Sprintf("Creating %s", cutString(page.Title, 20, true)))
		tfsTask, err := tfsApi.CreateChildTask(ctx, page.Title, page.Description, 0, requirement, []string{"Tech", "TechBacklog", "Prime", "SMP", "Core"})
		// tfsTask, err := &workitemtracking.WorkItem{Id: ptr.FromInt(93043)}, err

		if err != nil {
			pterm.Error.Println(fmt.Sprintf("TFS Task NOT CREATED %s: %s", page.Title, err.Error()))
		} else {
			progressbar.UpdateTitle(fmt.Sprintf("Updating wiki page %s", cutString(page.Title, 20, true)))
			page.AddTfsTask(*tfsTask.Id)
			err = updateTechDebtWikiPage(wikiApi, page)

			if err != nil {
				pterm.Error.Println(fmt.Sprintf("Wiki page NOT UDPATED %s: %s", page.Title, err.Error()))
			} else {
				pterm.Success.Println(fmt.Sprintf("CREATED %s", page.Title))
			}
		}

		progressbar.Increment()
	}
	_, _ = progressbar.Stop()
	return nil
}

func parseTechDebtPages(ctx context.Context, pageIDs []string, api *goconfluence.API) ([]*techDebtPage, error) {
	var pages []*techDebtPage
	wg, _ := errgroup.WithContext(ctx)
	var m sync.Mutex
	guard := make(chan struct{}, 10)
	for _, childPageID := range pageIDs {
		id := childPageID
		wg.Go(func() error {
			guard <- struct{}{}
			defer func() {
				<-guard
			}()

			content, err := api.GetContentByID(id, goconfluence.ContentQuery{
				Expand: []string{
					"body.storage",
					"space",
					"version",
				},
			})
			if err != nil {
				return err
			}

			techDebt, err := wiki.ParseTechDebt(content)
			if err != nil {
				return err
			}

			m.Lock()
			pages = append(pages, &techDebtPage{
				TechDebt: &techDebt,
				content:  content,
			})
			m.Unlock()

			return nil
		})
	}

	err := wg.Wait()
	if err != nil {
		return nil, err
	}

	return pages, nil
}

func updateTechDebtWikiPage(api *goconfluence.API, page *techDebtPage) error {
	content := page.content
	updatedBody, err := page.GetUpdatedBody()
	if err != nil {
		return err
	}

	_, err = api.UpdateContent(&goconfluence.Content{
		ID:    content.ID,
		Type:  content.Type,
		Title: content.Title,
		Space: &goconfluence.Space{
			Key: content.Space.Key,
		},
		Body: goconfluence.Body{
			Storage: goconfluence.Storage{
				Value:          updatedBody,
				Representation: "storage",
			},
		},
		Version: &goconfluence.Version{
			Number: content.Version.Number + 1,
		},
	})

	return err
}

func requestConfirmationTechDebt() (bool, error) {
	pterm.DefaultHeader.
		WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgDefault)).
		WithTextStyle(pterm.NewStyle(pterm.FgCyan)).
		Print("Press ENTER to continue. Any other key for cancel.")

	_, key, err := keyboard.GetSingleKey()
	if err != nil {
		return false, err
	}

	if key != keyboard.KeyEnter {
		return false, nil
	}

	return true, nil
}

func previewTechDebtTasks(pages []*techDebtPage) {
	titleWidth, descriptionWidth := getTechDebtColumnsWidth()

	var tableData [][]string
	tableData = append(tableData, []string{"#", "Title", "Description"})
	for i, page := range pages {

		tableData = append(tableData, []string{
			fmt.Sprintf("%d", i+1),
			cutString(page.Title, titleWidth, false),
			cutString(page.Description, descriptionWidth, false),
		})
	}

	_ = pterm.DefaultTable.
		WithHasHeader().
		WithData(tableData).
		Render()
}

func getTechDebtColumnsWidth() (int, int) {
	/*
		#  | Title | Description
		01 | *     | *
	*/

	colSep := 3
	numberCol := 2
	fixedWidth := numberCol + colSep + /*title?*/ colSep /*description?*/
	totalWidth := pterm.GetTerminalWidth()
	availableWidth := totalWidth - fixedWidth
	titleWidth := int(float64(availableWidth) * 0.4)
	descriptionWidth := availableWidth - titleWidth
	return titleWidth, descriptionWidth
}

func archiveTechCommand(ctx context.Context) error {
	wikiParentID := int(archiveTechCmdFlagWikiParentPageID)
	wikiArchiveID := int(archiveTechCmdFlagWikiArchivePageID)

	wikiApi, err := wiki.NewClient()
	if err != nil {
		return err
	}

	var pageIDs []string
	searchResult, err := wikiApi.GetChildPages(strconv.Itoa(wikiParentID))
	if err != nil {
		return err
	}

	for _, page := range searchResult.Results {
		pageIDs = append(pageIDs, page.ID)
	}

	pages, err := parseTechDebtPages(ctx, pageIDs, wikiApi)
	if err != nil {
		return err
	}

	if len(pages) == 0 {
		fmt.Println("nothing to archive")
		return nil
	}

	tfsApi, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	tasks, err := getTechDebtTasks(ctx, pages, tfsApi)
	if err != nil {
		return err
	}

	pages = lo.Filter(pages, func(page *techDebtPage, i int) bool {
		pageTasks, ok := tasks[page.PageID]
		return ok && lo.EveryBy(pageTasks, func(wi *workitemtracking.WorkItem) bool {
			state, ok := (*wi.Fields)["System.State"]
			return ok && (state == "Closed" || state == "Resolved")
		})
	})

	if len(pages) == 0 {
		fmt.Println("nothing to archive")
		return nil
	}

	previewTechDebtTasks(pages)

	ok, err := requestConfirmationTechDebt()
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("canceled by user")
	}

	return archiveTechDebtPages(ctx, pages, wikiApi, strconv.Itoa(wikiArchiveID))
}

type techDebtTask struct {
	*wiki.TfsTask
	status bool
}

func getTechDebtTasks(ctx context.Context, pages []*techDebtPage, api *tfs.API) (map[string][]*workitemtracking.WorkItem, error) {
	workItems := make(map[string][]*workitemtracking.WorkItem)
	wg, _ := errgroup.WithContext(ctx)
	var m sync.Mutex
	guard := make(chan struct{}, 10)
	for _, page := range pages {
		pageID := page.PageID
		for _, task := range page.TfsTasks {
			itemID := task.ItemID
			wg.Go(func() error {
				guard <- struct{}{}
				defer func() {
					<-guard
				}()

				workItem, err := api.WiClient.Get(ctx, itemID)
				if err != nil {
					return err
				}

				m.Lock()

				pageWorItems := workItems[pageID]
				pageWorItems = append(pageWorItems, workItem)
				workItems[pageID] = pageWorItems

				m.Unlock()

				return nil
			})
		}
	}

	err := wg.Wait()
	if err != nil {
		return nil, err
	}

	return workItems, nil
}

func archiveTechDebtPages(ctx context.Context, pages []*techDebtPage, wikiApi *goconfluence.API, archivePageID string) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(pages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, page := range pages {
		progressbar.UpdateTitle(fmt.Sprintf("Archiving %s", cutString(page.Title, 20, true)))
		err = archiveTechDebtWikiPage(wikiApi, page, archivePageID)
		if err != nil {
			pterm.Error.Println(fmt.Sprintf("NOT ARCHIVED %s: %s", page.Title, err.Error()))
		} else {
			pterm.Success.Println(fmt.Sprintf("ARCHIVED %s", page.Title))
		}

		progressbar.Increment()
	}
	_, _ = progressbar.Stop()
	return nil
}

func archiveTechDebtWikiPage(api *goconfluence.API, page *techDebtPage, archivePageID string) error {
	content := page.content

	_, err := api.UpdateContent(&goconfluence.Content{
		ID:    content.ID,
		Type:  content.Type,
		Title: content.Title,
		Ancestors: []goconfluence.Ancestor{
			{ID: archivePageID},
		},
		Body: goconfluence.Body{
			Storage: goconfluence.Storage{
				Value:          content.Body.Storage.Value,
				Representation: "storage",
			},
		},
		Space: &goconfluence.Space{
			Key: content.Space.Key,
		},
		Version: &goconfluence.Version{
			Number: content.Version.Number + 1,
		},
	})

	return err
}
