package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"tasker/tfs"
	"tasker/wiki"

	"github.com/eiannone/keyboard"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/workitemtracking"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	goconfluence "github.com/virtomize/confluence-go-api"
)

var (
	// syncCmd represents the sync command
	syncCmd = &cobra.Command{
		Use:   "sync <Wiki page ID>",
		Short: "Syncs wiki with tfs",
		Long:  `Creates or updates tasks from wiki page in TFS and inserts tfs-macros into wiki page`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			wikiPageID, err := strconv.Atoi(args[0])
			cobra.CheckErr(err)

			err = syncCommand(cmd.Context(), wikiPageID)
			cobra.CheckErr(err)
		},
	}

	featureIDRegexp         = regexp.MustCompile(`\d+`)
	startedWithNumberRegexp = regexp.MustCompile(`^\d+`)

	syncCmdFlagfeatureWorkItemID  uint32
	syncCmdFlagskipExistsingTasks bool
	syncCmdFlagSkipNewTasks       bool
	syncCmdFlagNoTitleAutoPrefix  bool
	syncCmdFlagTitleCustomPrefix  string
	syncCmdFlagTags               []string
	syncCmdFlagPartNumber         uint32
)

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().Uint32VarP(&syncCmdFlagfeatureWorkItemID, "feature", "f", 0, "ID of TFS feature work item (in case wiki page title not contains it)")
	syncCmd.Flags().BoolVar(&syncCmdFlagskipExistsingTasks, "create-only", false, "Do not update existing tasks")
	syncCmd.Flags().BoolVar(&syncCmdFlagSkipNewTasks, "update-only", false, "Do not create new tasks")
	syncCmd.Flags().StringVar(&syncCmdFlagTitleCustomPrefix, "prefix", "", "Custom prefix for each task, ie 'Part 3. '")
	syncCmd.Flags().BoolVar(&syncCmdFlagNoTitleAutoPrefix, "no-auto-prefix", false, "Do not prepend each task with index prefix")
	syncCmd.Flags().StringSliceVarP(&tags, "tag", "t", []string{}, "Tags of the task. Can be separated by comma or specified multiple times.")
	syncCmd.Flags().Uint32VarP(&syncCmdFlagPartNumber, "part", "p", 0, "Table number (tasks part), if tasks split into multiples tables (parts)")
}

func syncCommand(ctx context.Context, wikiPageID int) error {
	api, err := wiki.NewClient()
	if err != nil {
		return err
	}

	content, err := api.GetContentByID(strconv.Itoa(wikiPageID), goconfluence.ContentQuery{
		Expand: []string{
			"body.storage",
			"space",
			"version",
		},
	})

	if err != nil {
		return err
	}

	if syncCmdFlagfeatureWorkItemID <= 0 {
		id, err := strconv.ParseUint(featureIDRegexp.FindString(content.Title), 10, 32)
		if err != nil {
			return errors.New("unable to determine TFS feature ID from wiki page title")
		}
		syncCmdFlagfeatureWorkItemID = uint32(id)
	}

	tasks, err := wiki.ParseTasks(content.Body.Storage.Value)
	if err != nil {
		return err
	}

	tables, err := groupByTable(tasks)
	if err != nil {
		return err
	}

	for _, table := range tables {
		addTitlePrefixes(table, len(tables) > 1)
	}

	if syncCmdFlagPartNumber > 0 {
		if int(syncCmdFlagPartNumber) > len(tables) {
			return errors.New("invalid table (part) number")
		}
		table := tables[syncCmdFlagPartNumber-1]
		tasks = table.Tasks
	}

	tasks = filterTasks(tasks, func(t *wiki.Task) bool {
		switch {
		case syncCmdFlagSkipNewTasks && t.TfsTaskID == 0:
			return false
		case syncCmdFlagskipExistsingTasks && t.TfsTaskID != 0:
			return false
		default:
			return true
		}
	})

	if len(tasks) == 0 {
		fmt.Println("nothing to create or update")
		return nil
	}

	// remove empty and not selected tables by grouping remained tasks again
	tables, _ = groupByTable(tasks)

	err = requestConfirmation(tables)
	if err != nil {
		return err
	}

	err = createTasks(ctx, int(syncCmdFlagfeatureWorkItemID), tasks)
	if err != nil {
		return err
	}

	err = updateWikiPage(api, content, tasks)

	return err
}

func filterTasks(tasks []*wiki.Task, predicate func(*wiki.Task) bool) []*wiki.Task {
	var filtered []*wiki.Task
	for _, t := range tasks {
		if predicate(t) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func updateWikiPage(api *goconfluence.API, content *goconfluence.Content, tasks []*wiki.Task) error {
	spinner, _ := pterm.DefaultSpinner.WithText("Updating wiki page...").Start()

	body := content.Body.Storage.Value
	updatedBody, modified, err := wiki.UpdatePageContent(body, tasks)
	if err != nil {
		return err
	}

	if !modified {
		spinner.Success("Wiki page not changed")
		return nil
	}

	_, err = api.UpdateContent(&goconfluence.Content{
		ID:    content.ID,
		Type:  content.Type,
		Title: content.Title,
		Space: goconfluence.Space{
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

	if err == nil {
		spinner.Success("Wiki page updated")
	} else {
		spinner.Fail("Wiki pdage not updated: " + err.Error())
	}
	_ = spinner.Stop()
	return err
}

func createTasks(ctx context.Context, featureID int, tasks []*wiki.Task) error {
	progressbar, err := pterm.DefaultProgressbar.WithTitle("Processing...").WithTotal(len(tasks)).WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}

	a, err := tfs.NewAPI(ctx)
	if err != nil {
		return err
	}

	feature, err := a.Client.Get(ctx, featureID)
	if err != nil {
		return err
	}

	for _, t := range tasks {
		progressbar.UpdateTitle(fmt.Sprintf("Creating %s", cutString(t.Title, 20, true)))

		if t.TfsTaskID > 0 {
			err := a.Client.Update(ctx, t.TfsTaskID, t.Title, t.Description, t.Estimate)
			if err == nil {
				pterm.Success.Println(fmt.Sprintf("UPDATED %s", t.Title))
			} else {
				pterm.Warning.Println(fmt.Sprintf("NOT UPDATED %s: %s", t.Title, err.Error()))
			}
		} else {
			tfsTask, err := a.CreateFeatureTask(ctx, t.Title, t.Description, t.Estimate, feature, syncCmdFlagTags)
			if err == nil {
				pterm.Success.Println(fmt.Sprintf("CREATED %s", t.Title))
				t.Update(createTfsTaskMacros(tfsTask))
			} else {
				pterm.Error.Println(fmt.Sprintf("NOT CREATED %s: %s", t.Title, err.Error()))
			}
		}

		progressbar.Increment()
	}
	_, _ = progressbar.Stop()

	return nil
}

func createTfsTaskMacros(task *workitemtracking.WorkItem) string {
	return `<div class="content-wrapper">
			<p>
				<ac:structured-macro ac:name="work-item-tfs" ac:schema-version="1" ac:macro-id="` + uuid.NewString() + `">
					<ac:parameter ac:name="itemID">` + strconv.Itoa(*task.Id) + `</ac:parameter>
					<ac:parameter ac:name="host">1</ac:parameter>
					<ac:parameter ac:name="assigned">true</ac:parameter>
					<ac:parameter ac:name="title">false</ac:parameter>
					<ac:parameter ac:name="type">false</ac:parameter>
					<ac:parameter ac:name="status">true</ac:parameter>
				</ac:structured-macro>
			</p>
		</div>`
}

func addTitlePrefixes(table *Table, withPartNumber bool) {
	for i, t := range table.Tasks {
		if !syncCmdFlagNoTitleAutoPrefix && !startedWithNumberRegexp.MatchString(t.Title) {
			t.Title = fmt.Sprintf("%02d. %s", i+1, t.Title)
		}

		if len(syncCmdFlagTitleCustomPrefix) > 0 {
			t.Title = fmt.Sprintf("%s%s", syncCmdFlagTitleCustomPrefix, t.Title)
		} else if !syncCmdFlagNoTitleAutoPrefix && withPartNumber {
			t.Title = fmt.Sprintf("%d.%s", table.Number, t.Title)
		}
	}
}

func requestConfirmation(tables []*Table) error {
	var tasksTotalCount int
	for _, table := range tables {
		if len(tables) > 1 {
			pterm.DefaultSection.Printfln("Part %d", table.Number)
		}

		previewTasks(table.Tasks)
		tasksTotalCount += len(table.Tasks)
	}

	if len(tables) > 1 {
		pterm.DefaultSection.Printfln("Total tasks: %d", tasksTotalCount)
	}

	pterm.DefaultHeader.
		WithFullWidth().
		WithBackgroundStyle(pterm.NewStyle(pterm.BgDefault)).
		WithTextStyle(pterm.NewStyle(pterm.FgCyan)).
		Print("Press ENTER to continue. Any other key for cancel.")

	_, key, err := keyboard.GetSingleKey()
	if err != nil {
		return err
	}

	if key != keyboard.KeyEnter {
		return errors.New("canceled by user")
	}

	return nil
}

type Table struct {
	Number int
	Index  int
	Tasks  []*wiki.Task
}

func groupByTable(tasks []*wiki.Task) ([]*Table, error) {
	var tables []*Table
	m := make(map[int]*Table)
	for _, t := range tasks {
		tableIndex := t.TableIndex()

		if tableIndex == -1 {
			return nil, errors.New("table index not found")
		}

		table, ok := m[tableIndex]
		if !ok {
			table = &Table{
				Number: len(tables) + 1,
				Index:  tableIndex,
			}
			tables = append(tables, table)
			m[tableIndex] = table
		}

		table.Tasks = append(table.Tasks, t)
	}

	return tables, nil
}

func previewTasks(tasks []*wiki.Task) {
	titleWidth, descriptionWidth := getColumnsWidth()

	var tableData [][]string
	tableData = append(tableData, []string{"#", "Title", "Description", "Estimate", "TFS"})
	for i, task := range tasks {

		tfsTaskID := ""
		switch {
		case task.TfsTaskID < 0:
			tfsTaskID = "n/a"
		case task.TfsTaskID == 0:
		default:
			tfsTaskID = fmt.Sprintf("%d", task.TfsTaskID)
		}

		tableData = append(tableData, []string{
			fmt.Sprintf("%d", i+1),
			cutString(task.Title, titleWidth, false),
			cutString(task.Description, descriptionWidth, false),
			fmt.Sprintf("%v", task.Estimate),
			tfsTaskID,
		})
	}

	_ = pterm.DefaultTable.
		WithHasHeader().
		WithData(tableData).
		Render()
}

func cutString(value string, maxLength int, padded bool) string {
	runeCount := utf8.RuneCountInString(value)
	if runeCount > maxLength {
		runes := []rune(value)
		return string(runes[:maxLength-3]) + "..."
	}
	if padded && runeCount < maxLength {
		return value + strings.Repeat(" ", maxLength-runeCount)
	}
	return value
}

func getColumnsWidth() (int, int) {
	/*
		#  | Title | Description  | Estimate | TFS
		01 | *     | *            | 3        | 12345
	*/

	colSep := 3
	numberCol := 2
	estimateCol := 8
	tfsCol := 5
	fixedWidth := numberCol + colSep + /*title?*/ colSep + /*description?*/ colSep + estimateCol + colSep + tfsCol
	totalWidth := pterm.GetTerminalWidth()
	availableWidth := totalWidth - fixedWidth
	titleWidth := int(float64(availableWidth) * 0.4)
	descriptionWidth := availableWidth - titleWidth
	return titleWidth, descriptionWidth
}
