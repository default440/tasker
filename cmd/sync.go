package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode/utf8"

	"tasker/tasksui"
	"tasker/tfs"
	"tasker/wiki"

	"github.com/eiannone/keyboard"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/workitemtracking"
	"github.com/pterm/pterm"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	syncCmdFlagFeatureWorkItemID uint32
	syncCmdFlagSkipExistingTasks bool
	syncCmdFlagSkipNewTasks      bool
	syncCmdFlagNoTitleAutoPrefix bool
	syncCmdFlagTitleCustomPrefix string
	syncCmdFlagTags              []string
	syncCmdFlagPartNumber        uint32
	syncCmdFlagAppendTagsToTitle bool

	syncCmdTemplatesCache = make(map[string]*template.Template)
)

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().Uint32VarP(&syncCmdFlagFeatureWorkItemID, "feature", "f", 0, "ID of TFS feature work item (in case wiki page title not contains it)")
	syncCmd.Flags().BoolVar(&syncCmdFlagSkipExistingTasks, "create-only", false, "Do not update existing tasks")
	syncCmd.Flags().BoolVar(&syncCmdFlagSkipNewTasks, "update-only", false, "Do not create new tasks")
	syncCmd.Flags().StringVar(&syncCmdFlagTitleCustomPrefix, "prefix", "", "Custom prefix for each task, ie \"Part 3. \"")
	syncCmd.Flags().BoolVar(&syncCmdFlagNoTitleAutoPrefix, "no-auto-prefix", false, "Do not prepend each task with index prefix")
	syncCmd.Flags().StringSliceVarP(&syncCmdFlagTags, "tag", "t", []string{"разработка"}, "Tags of the tasks. Can be separated by comma or specified multiple times.")
	syncCmd.Flags().Uint32VarP(&syncCmdFlagPartNumber, "part", "p", 0, "Table number (tasks part), if tasks splitted into multiple tables (parts)")
	syncCmd.Flags().BoolVar(&syncCmdFlagAppendTagsToTitle, "append-tags-to-title", false, "Append tas tags to task title")
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

	if syncCmdFlagFeatureWorkItemID <= 0 {
		id, err := strconv.ParseUint(featureIDRegexp.FindString(content.Title), 10, 32)
		if err != nil {
			return errors.New("unable to determine TFS feature ID from wiki page title")
		}
		syncCmdFlagFeatureWorkItemID = uint32(id)
	}

	tasks, err := wiki.ParseTasksTable(content.Body.Storage.Value)
	if err != nil {
		return err
	}

	tables, err := wiki.GroupByTable(tasks)
	if err != nil {
		return err
	}

	for _, table := range tables {
		addTitlePrefixes(table, len(tables) > 1)
		for _, t := range table.Tasks {
			t.Tags = append(t.Tags, syncCmdFlagTags...)
		}
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
		case syncCmdFlagSkipExistingTasks && t.TfsTaskID != 0:
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
	tables, _ = wiki.GroupByTable(tasks)

	uiTables := lo.Map(tables, func(tbl *wiki.Table, _ int) tasksui.Table {
		return tbl
	})

	ok, err := tasksui.PreviewTasks(uiTables)
	if err != nil {
		return err
	}

	if ok {
		err = createTasks(ctx, int(syncCmdFlagFeatureWorkItemID), tasks)
		if err != nil {
			return err
		}

		err = updateWikiPage(api, content, tasks)
	}

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

func updateWikiPage(api *wiki.API, content *goconfluence.Content, tasks []*wiki.Task) error {
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

	if err == nil {
		spinner.Success("Wiki page updated")
	} else {
		spinner.Fail("Wiki page not updated: " + err.Error())
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

	feature, err := a.WiClient.Get(ctx, featureID)
	if err != nil {
		return err
	}

	for _, t := range tasks {
		title := t.Title
		if syncCmdFlagAppendTagsToTitle {
			for _, tag := range t.Tags {
				title = fmt.Sprintf("[%s] %s", tag, title)
			}
		}

		progressbar.UpdateTitle(fmt.Sprintf("Creating %s", cutString(title, 20, true)))

		if t.TfsTaskID > 0 {
			err := a.WiClient.Update(ctx, t.TfsTaskID, title, t.Description, t.Estimate)
			if err == nil {
				pterm.Success.Println(fmt.Sprintf("UPDATED %s", title))
			} else {
				pterm.Warning.Println(fmt.Sprintf("NOT UPDATED %s: %s", title, err.Error()))
			}
		} else {
			tfsTask, err := a.CreateChildTask(ctx, title, t.Description, t.Estimate, feature, t.Tags, t.AssignedTo)
			if err == nil {
				pterm.Success.Println(fmt.Sprintf("CREATED %s", title))
				t.Update(createTfsTaskMacro(tfsTask))
			} else {
				pterm.Error.Println(fmt.Sprintf("NOT CREATED %s: %s", title, err.Error()))
			}
		}

		progressbar.Increment()
	}
	_, _ = progressbar.Stop()

	return nil
}

type syncCmdTfsTaskMacroTemplateData struct {
	Task *workitemtracking.WorkItem
}

func (t syncCmdTfsTaskMacroTemplateData) NewUUID() string {
	return uuid.NewString()
}

func createTfsTaskMacro(task *workitemtracking.WorkItem) string {
	tfsTaskMacroPath := viper.GetString("syncCmdTfsTaskMacroPath")

	if strings.TrimSpace(tfsTaskMacroPath) != "" {
		t, ok := syncCmdTemplatesCache[tfsTaskMacroPath]
		if !ok {
			var err error
			t, err = template.New("sync cmd tfs task macro template").ParseFiles(tfsTaskMacroPath)
			if err != nil {
				log.Fatalf("%v\n", err)
			}
			syncCmdTemplatesCache[tfsTaskMacroPath] = t
		}

		var result bytes.Buffer
		err := t.ExecuteTemplate(&result, t.Templates()[0].Name(), syncCmdTfsTaskMacroTemplateData{
			Task: task,
		})
		if err != nil {
			log.Fatalf("%v\n", err)
		}

		return result.String()
	}

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

func addTitlePrefixes(table *wiki.Table, withPartNumber bool) {
	for i, t := range table.Tasks {
		if !syncCmdFlagNoTitleAutoPrefix && !startedWithNumberRegexp.MatchString(t.Title) {
			t.Title = fmt.Sprintf("%02d. %s", i+1, t.Title)
		}

		if !syncCmdFlagNoTitleAutoPrefix && withPartNumber {
			t.Title = fmt.Sprintf("%d.%s", table.Number, t.Title)
		}

		if len(syncCmdFlagTitleCustomPrefix) > 0 {
			t.Title = fmt.Sprintf("%s%s", syncCmdFlagTitleCustomPrefix, t.Title)
		} else {
			t.Title = fmt.Sprintf("%s", t.Title)
		}
	}
}

func requestConfirmation(tables []*wiki.Table) error {
	var tasksTotalCount int
	var totalEstimate int
	for _, table := range tables {
		if len(tables) > 1 {
			pterm.DefaultSection.Printfln("Part %d", table.Number)
		}

		previewTasks(table.Tasks)

		tasksTotalCount += len(table.Tasks)
		for _, t := range table.Tasks {
			totalEstimate += int(t.Estimate)
		}
	}

	if len(tables) > 1 {
		println("")
		pterm.DefaultBox.WithTitle("Total").Printfln("Tasks: %d\nEstimate: %v", tasksTotalCount, totalEstimate)
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

func previewTasks(tasks []*wiki.Task) {
	titleWidth, descriptionWidth := getColumnsWidth()

	var totalEstimate float32
	var tableData [][]string
	tableData = append(tableData, []string{"#", "Title", "Description", "Estimate", "TFS"})
	for i, task := range tasks {
		totalEstimate += task.Estimate

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

	tableData = append(tableData, []string{"", "∑", "", fmt.Sprintf("%v", totalEstimate), ""})

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
