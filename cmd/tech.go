package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"tasker/tasksui"
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
		Long:  `View, create, synchronize etc. technical debt tasks.`,
	}

	syncTechCmd = &cobra.Command{
		Use:   "sync",
		Short: "Sync tech debt tasks",
		Long:  `Synchronize technical debt tasks between TFS and Wiki.`,
		Run: func(cmd *cobra.Command, _ []string) {
			err := syncTechCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	archiveTechCmd = &cobra.Command{
		Use:     "archive",
		Aliases: []string{"arch"},
		Short:   "Archive completed tech debt tasks",
		Long:    `Move completed technical debt tasks under Archive page.`,
		Run: func(cmd *cobra.Command, _ []string) {
			err := archiveTechCommand(cmd.Context())
			cobra.CheckErr(err)
		},
	}

	techDebtPriorityRegexp   = regexp.MustCompile(`\d+`)
	techDebtEstimationRegexp = regexp.MustCompile(`\[\d+\]`)

	syncTechCmdFlagTfsRequirementID    uint
	syncTechCmdFlagWikiParentPageID    uint
	syncTechCmdFlagWikiTechDebtPageIDs []uint
	syncTechCmdFlagTfsWorkItemType     string
	syncTechCmdFlagForceCreate         bool
	syncTechCmdFlagTfsWorkItemPrefix   string
	syncTechCmdFlagTfsEstimate         uint
	syncTechCmdFlagTfsDefaultPriority  uint

	archiveTechCmdFlagWikiParentPageID  uint
	archiveTechCmdFlagWikiArchivePageID uint
)

func init() {
	rootCmd.AddCommand(techCmd)
	techCmd.AddCommand(syncTechCmd)
	techCmd.AddCommand(archiveTechCmd)

	syncTechCmd.Flags().UintVarP(&syncTechCmdFlagTfsRequirementID, "requirement", "r", 0, "The ID of Parent TFS requirement/feature work item for Tech Debt tasks")
	syncTechCmd.Flags().UintVarP(&syncTechCmdFlagWikiParentPageID, "parent-page", "p", 0, "The ID of Wiki parent page with Tech Debt tasks")
	syncTechCmd.Flags().UintSliceVarP(&syncTechCmdFlagWikiTechDebtPageIDs, "debt-page", "d", []uint{}, "The ID of Wiki page with Tech Debt task")
	syncTechCmd.Flags().StringVarP(&syncTechCmdFlagTfsWorkItemType, "type", "", "Requirement", "The type of TFS new Work Item (Task, Requirement, etc)")
	syncTechCmd.Flags().BoolVarP(&syncTechCmdFlagForceCreate, "force", "", false, "Create work items even though there is already a link to the task on the wiki page.")
	syncTechCmd.Flags().StringVarP(&syncTechCmdFlagTfsWorkItemPrefix, "prefix", "", "[SMP] [tech]", "The prefix of work items")
	syncTechCmd.Flags().UintVarP(&syncTechCmdFlagTfsEstimate, "estimate", "e", 16, "The default estimate")
	syncTechCmd.Flags().UintVarP(&syncTechCmdFlagTfsDefaultPriority, "priority", "", 1, "The work item default priority")

	cobra.CheckErr(syncTechCmd.MarkFlagRequired("requirement"))

	archiveTechCmd.Flags().UintVarP(&archiveTechCmdFlagWikiParentPageID, "parent-page", "p", 0, "ID of Wiki parent page with Tech Debt tasks")
	archiveTechCmd.Flags().UintVarP(&archiveTechCmdFlagWikiArchivePageID, "archive-page", "a", 0, "ID of Wiki archive page with completed Tech Debt tasks")

	cobra.CheckErr(archiveTechCmd.MarkFlagRequired("parent-page"))
	cobra.CheckErr(archiveTechCmd.MarkFlagRequired("archive-page"))
}

type techDebtPage struct {
	*wiki.TechDebt
	content  *goconfluence.Content
	estimate float32
	priority float32
}

func (t *techDebtPage) GetTitle() string                  { return t.Title }
func (t *techDebtPage) SetTitle(title string)             { t.Title = title }
func (t *techDebtPage) GetDescription() string            { return t.Description }
func (t *techDebtPage) SetDescription(description string) { t.Description = description }
func (t *techDebtPage) GetEstimate() float32              { return t.estimate }
func (t *techDebtPage) SetEstimate(estimate float32)      { t.estimate = estimate }
func (t *techDebtPage) GetPriority() float32              { return t.priority }
func (t *techDebtPage) SetPriority(priority float32)      { t.priority = priority }
func (t *techDebtPage) GetTfsTaskID() int                 { return 0 }
func (t *techDebtPage) SetTfsTaskID(_ int)                {}
func (t *techDebtPage) Clone() tasksui.Task {
	t2 := *t
	return &t2
}

type techDebtPageTable struct {
	pages []*techDebtPage
}

func (t *techDebtPageTable) GetTasks() []tasksui.Task {
	return lo.Map(t.pages, func(page *techDebtPage, _ int) tasksui.Task { return page })
}
func (t *techDebtPageTable) SetTask(tsk tasksui.Task, index int) {
	t.pages[index] = tsk.(*techDebtPage)
}

func syncTechCommand(ctx context.Context) error {
	wikiParentID := int(syncTechCmdFlagWikiParentPageID)
	requirementID := int(syncTechCmdFlagTfsRequirementID)

	wikiAPI, err := wiki.NewClient()
	if err != nil {
		return err
	}

	var pageIDs []string
	if wikiParentID != 0 {
		searchResult, err := wikiAPI.GetChildPages(strconv.Itoa(wikiParentID))
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

	pages, err := parseTechDebtPages(ctx, pageIDs, wikiAPI)
	if err != nil {
		return err
	}

	if !syncTechCmdFlagForceCreate {
		pages = lo.Filter(pages, func(item *techDebtPage, _ int) bool {
			return len(item.TfsTasks) == 0 && !item.IsEmptyPage
		})
	}

	if len(pages) == 0 {
		fmt.Println("nothing to create or update")
		return nil
	}

	tfsAPI, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	requirement, err := tfsAPI.WiClient.Get(ctx, requirementID)
	if err != nil {
		return err
	}

	uiTables := []tasksui.Table{
		&techDebtPageTable{pages: pages},
	}

	for _, page := range pages {
		page.Title, _ = strings.CutPrefix(page.Title, fmt.Sprintf("%v.", page.priority))
		page.Title, _ = strings.CutSuffix(page.Title, fmt.Sprintf("[%v]", page.estimate))
		page.Title = strings.TrimSpace(page.Title)
	}

	if syncTechCmdFlagTfsWorkItemPrefix != "" {
		for _, page := range pages {
			page.Title = fmt.Sprintf("%s %s", syncTechCmdFlagTfsWorkItemPrefix, page.Title)
		}
	}

	ok, err := tasksui.PreviewTasks(uiTables)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	_ = requirement

	err = createTechDebtTasks(ctx, pages, tfsAPI, wikiAPI, requirement)
	if err != nil {
		return err
	}

	return nil
}

func createTechDebtTasks(ctx context.Context, pages []*techDebtPage, tfsAPI *tfs.API, wikiAPI *wiki.API, requirement *workitemtracking.WorkItem) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(pages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, page := range pages {
		progressbar.UpdateTitle(fmt.Sprintf("Creating %s", cutString(page.Title, 20, true)))
		tags := []string{}
		tags = append(tags, page.Labels...)

		var tfsTask *workitemtracking.WorkItem
		switch syncTechCmdFlagTfsWorkItemType {
		case "Task":
			tfsTask, err = tfsAPI.CreateChildTask(ctx, page.Title, page.Description, page.estimate, requirement, tags)
		case "Requirement":
			tfsTask, err = tfsAPI.CreateChildRequirement(ctx, "Technical", page.Title, page.Description, page.estimate, page.priority, requirement, tags)
		default:
			return fmt.Errorf("unknown work item type: %s", syncTechCmdFlagTfsWorkItemType)
		}

		if err != nil {
			pterm.Error.Println(fmt.Sprintf("TFS Task NOT CREATED %s: %s", page.Title, err.Error()))
		} else {
			progressbar.UpdateTitle(fmt.Sprintf("Updating wiki page %s", cutString(page.Title, 20, true)))
			page.AddTfsTask(*tfsTask.Id)
			err = updateTechDebtWikiPage(wikiAPI, page)

			if err != nil {
				pterm.Error.Println(fmt.Sprintf("Wiki page NOT UPDATED %s: %s", page.Title, err.Error()))
			} else {
				pterm.Success.Println(fmt.Sprintf("CREATED %s", page.Title))
			}
		}

		progressbar.Increment()
	}
	_, _ = progressbar.Stop()
	return nil
}

func parseTechDebtPages(ctx context.Context, pageIDs []string, api *wiki.API) ([]*techDebtPage, error) {
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

			labels, err := api.GetLabels(id)
			if err != nil {
				return err
			}

			priority, err := strconv.ParseFloat(techDebtPriorityRegexp.FindString(content.Title), 32)
			if err != nil {
				priority = float64(syncTechCmdFlagTfsDefaultPriority)
			}

			estimation, err := strconv.ParseFloat(strings.Trim(techDebtEstimationRegexp.FindString(content.Title), "[]"), 32)
			if err != nil {
				estimation = float64(syncTechCmdFlagTfsEstimate)
			}

			techDebt, err := wiki.ParseTechDebt(content)
			if err != nil {
				return err
			}

			techDebt.Labels = lo.Map(labels.Labels, func(label goconfluence.Label, _ int) string {
				return label.Name
			})

			m.Lock()
			pages = append(pages, &techDebtPage{
				TechDebt: &techDebt,
				content:  content,
				estimate: float32(estimation),
				priority: float32(priority),
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

func updateTechDebtWikiPage(api *wiki.API, page *techDebtPage) error {
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

	wikiAPI, err := wiki.NewClient()
	if err != nil {
		return err
	}

	var pageIDs []string
	searchResult, err := wikiAPI.GetChildPages(strconv.Itoa(wikiParentID))
	if err != nil {
		return err
	}

	for _, page := range searchResult.Results {
		pageIDs = append(pageIDs, page.ID)
	}

	pages, err := parseTechDebtPages(ctx, pageIDs, wikiAPI)
	if err != nil {
		return err
	}

	if len(pages) == 0 {
		fmt.Println("nothing to archive")
		return nil
	}

	tfsAPI, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	tasks, err := getTechDebtTasks(ctx, pages, tfsAPI)
	if err != nil {
		return err
	}

	pages = lo.Filter(pages, func(page *techDebtPage, _ int) bool {
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

	return archiveTechDebtPages(pages, wikiAPI, strconv.Itoa(wikiArchiveID))
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

func archiveTechDebtPages(pages []*techDebtPage, wikiAPI *wiki.API, archivePageID string) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(pages)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	for _, page := range pages {
		progressbar.UpdateTitle(fmt.Sprintf("Archiving %s", cutString(page.Title, 20, true)))
		err := wikiAPI.MovePage(page.content.Space.Key, page.PageID, archivePageID)
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
