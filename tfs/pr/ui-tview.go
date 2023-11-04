package pr

import (
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/pterm/pterm"
	"github.com/rivo/tview"
	"golang.org/x/exp/slices"
)

type TviewUI struct {
	repositoryChangeHandler   func(repository string)
	targetBranchChangeHandler func(targetBranch git.GitBranchStats)
	sourceBranchChangeHandler func(sourceBranch git.GitBranchStats)
	createHandler             func(selections UserSelections)
	cancelHandler             func()
	errHandler                func(error)

	repositories []string

	app                  *tview.Application
	grid                 *tview.Grid
	repositoriesDropDown *tview.DropDown
	sourceBranchDropDown *tview.DropDown
	targetBranchDropDown *tview.DropDown
	workItemsInput       *tview.InputField
	mergeMessageInput    *tview.TextArea
	errorTextView        *tview.TextView
	helpTextView         *tview.TextView

	sourceBranch    git.GitBranchStats
	targetBranch    git.GitBranchStats
	repository      string
	workItems       []string
	squash          bool
	withWorkItemIDs bool
	mergeMessage    string
}

func NewTviewUI() (*TviewUI, error) {
	ui := TviewUI{
		squash:          true,
		withWorkItemIDs: true,
	}

	app := tview.NewApplication()

	grid := tview.NewGrid()
	grid.SetRows(0, 1)
	ui.helpTextView = tview.NewTextView().SetText(" Press Ctrl+S for save, press Ctrl+C or ESC to exit")
	grid.AddItem(ui.helpTextView, 1, 0, 1, 1, 0, 0, false)

	form := tview.NewForm()
	form.SetBorder(true)
	grid.AddItem(form, 0, 0, 1, 1, 0, 0, true)

	ui.app = app
	ui.grid = grid
	ui.repositoriesDropDown = tview.NewDropDown().SetLabel("Repository")
	ui.sourceBranchDropDown = tview.NewDropDown().SetLabel("Source branch")
	ui.targetBranchDropDown = tview.NewDropDown().SetLabel("Target branch")

	ui.workItemsInput = tview.NewInputField().
		SetLabel("Work items").
		SetFieldWidth(40).
		SetAcceptanceFunc(func(textToCheck string, lastChar rune) bool {
			matched, _ := regexp.MatchString(workItemsInputRegexp, textToCheck)
			return matched
		}).
		SetChangedFunc(func(text string) {
			ui.workItems = parseWorkItemIDs([]string{text})
		})

	ui.mergeMessageInput = tview.NewTextArea().
		SetLabel("Merge commit message").
		SetSize(tview.DefaultFormFieldHeight, 80).
		SetMaxLength(0).
		SetChangedFunc(func() {
			ui.mergeMessage = ui.mergeMessageInput.GetText()
		})

	form.AddFormItem(ui.repositoriesDropDown)
	form.AddFormItem(ui.sourceBranchDropDown)
	form.AddFormItem(ui.targetBranchDropDown)
	form.AddFormItem(ui.mergeMessageInput)
	form.AddFormItem(ui.workItemsInput)
	form.AddCheckbox("Squash pr?", ui.squash, func(checked bool) {
		ui.squash = checked
	})
	form.AddCheckbox("Prepend work item IDs to commit message?", ui.withWorkItemIDs, func(withWorkItemIDs bool) {
		ui.withWorkItemIDs = withWorkItemIDs
	})
	form.AddButton("Create", func() {
		ui.execCreateHandler()
	})
	form.AddButton("Cancel", func() {
		ui.execCancelHandler()
	})

	app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyCtrlS:
			ui.execCreateHandler()
		case tcell.KeyCtrlC:
			ui.execCancelHandler()
		}

		return ev
	})

	go func() {
		width := 130
		heigth := 30
		var p tview.Primitive = grid
		if pterm.GetTerminalWidth() > width && pterm.GetTerminalHeight() > heigth {
			p = center(width, heigth, grid)
		}

		if err := app.SetRoot(p, true).EnableMouse(true).Run(); err != nil {
			ui.execErrHandler(err)
		}
		ui.execCancelHandler()
	}()

	return &ui, nil
}

func center(width, height int, p tview.Primitive) *tview.Flex {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(tview.NewBox(), 0, 1, false), width, 1, true).
		AddItem(tview.NewBox(), 0, 1, false)
}

func (ui *TviewUI) execErrHandler(err error) {
	handler := ui.errHandler
	if handler != nil {
		handler(err)
	}
}

func (ui *TviewUI) Stop() {
	ui.app.Stop()
}

func (ui *TviewUI) execCancelHandler() {
	ui.Stop()
	handler := ui.cancelHandler
	if handler != nil {
		handler()
	}
}

func (ui *TviewUI) execCreateHandler() {
	handler := ui.createHandler
	if handler != nil {
		result := UserSelections{
			SourceBranch:    &ui.sourceBranch,
			TargetBranch:    &ui.targetBranch,
			Repository:      ui.repository,
			WorkItems:       ui.workItems,
			MergeMessage:    ui.mergeMessage,
			Squash:          ui.squash,
			WithWorkItemIDs: ui.withWorkItemIDs,
		}
		handler(result)
	}
}

func (ui *TviewUI) SetRepositories(repositories []string) {
	ui.repositories = repositories
	ui.repositoriesDropDown.
		SetOptions(repositories, func(text string, index int) {
			ui.repository = text
			ui.execRepositoryChangeHandler(text)
		})

	if len(repositories) > 0 {
		ui.repositoriesDropDown.SetCurrentOption(0)
	}

	ui.app.Draw()
}

func (ui *TviewUI) SetError(error string) {
	if ui.errorTextView == nil {
		ui.errorTextView = tview.NewTextView()
		ui.errorTextView.SetBackgroundColor(tcell.ColorOrangeRed)
		ui.grid.SetRows(0, 1, 1)
		ui.grid.AddItem(ui.errorTextView, 1, 0, 1, 1, 0, 0, false)
		ui.grid.AddItem(ui.helpTextView, 2, 0, 1, 1, 0, 0, false)
	}

	ui.errorTextView.SetText(" " + error)
}

func (ui *TviewUI) SetRepository(repository string) {
	i := slices.Index(ui.repositories, repository)
	if i >= 0 {
		ui.repositoriesDropDown.SetCurrentOption(i)
		ui.app.Draw()
	}
}

func (ui *TviewUI) execRepositoryChangeHandler(repository string) {
	handler := ui.repositoryChangeHandler
	if handler != nil {
		handler(repository)
	}
}

func (ui *TviewUI) SetSourceBranches(branches []git.GitBranchStats) {
	options := getBranchNames(branches)
	ui.sourceBranchDropDown.
		SetOptions(options, func(text string, index int) {
			ui.sourceBranch = branches[index]
			handler := ui.sourceBranchChangeHandler
			if handler != nil {
				handler(ui.sourceBranch)
			}
		})

	if len(options) > 0 {
		ui.sourceBranchDropDown.SetCurrentOption(0)
	}
}

func (ui *TviewUI) SetTargetBranches(branches []git.GitBranchStats) {
	options := getBranchNames(branches)
	ui.targetBranchDropDown.
		SetOptions(options, func(text string, index int) {
			ui.targetBranch = branches[index]
			handler := ui.targetBranchChangeHandler
			if handler != nil {
				handler(ui.targetBranch)
			}
		})

	if len(options) > 0 {
		ui.targetBranchDropDown.SetCurrentOption(0)
	}
}

func (ui *TviewUI) SetWorkItems(workItemIDs []string) {
	initialValue := strings.Join(workItemIDs, ", ")
	ui.workItemsInput.SetText(initialValue)
}

func (ui *TviewUI) SetMergeMessage(mergeMessage string) {
	ui.mergeMessageInput.SetText(mergeMessage, true)
}

func getBranchNames(branches []git.GitBranchStats) []string {
	options := make([]string, 0, len(branches))
	for i := 0; i < len(branches); i++ {
		options = append(options, *branches[i].Name)
	}
	return options
}

func (ui *TviewUI) SetRepositoryChangeHandler(handler func(repository string)) {
	ui.repositoryChangeHandler = handler
}

func (ui *TviewUI) SetTargetBranchChangeHandler(handler func(targetBranch git.GitBranchStats)) {
	ui.targetBranchChangeHandler = handler
}

func (ui *TviewUI) SetSourceBranchChangeHandler(handler func(sourceBranch git.GitBranchStats)) {
	ui.sourceBranchChangeHandler = handler
}

func (ui *TviewUI) SetCreateHandler(handler func(selections UserSelections)) {
	ui.createHandler = handler
}

func (ui *TviewUI) SetCancelHandler(handler func()) {
	ui.cancelHandler = handler
}

func (ui *TviewUI) SetErrHandler(handler func(error)) {
	ui.errHandler = handler
}
