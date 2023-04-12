package pr

import (
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/git"
	"github.com/rivo/tview"
	"golang.org/x/exp/slices"
)

type tviewUI struct {
	repositoryChangeHandler   func(repository string)
	targetBranchChangeHandler func(targetBranch git.GitBranchStats)
	sourceBranchChangeHandler func(sourceBranch git.GitBranchStats)
	createHandler             func(selections UserSelections)
	cancelHandler             func()
	errHandler                func(error)

	repositories []string

	app                  *tview.Application
	repositoriesDropDown *tview.DropDown
	sourceBranchDropDown *tview.DropDown
	targetBranchDropDown *tview.DropDown
	workItemsInput       *tview.InputField
	mergeMessageInput    *tview.TextArea

	sourceBranch git.GitBranchStats
	targetBranch git.GitBranchStats
	repository   string
	workItems    []string
	squash       bool
	mergeMessage string
}

func NewTviewUI() (*tviewUI, error) {
	ui := tviewUI{}

	app := tview.NewApplication()
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Ctr+S create, Ctrl+C exit ")

	ui.app = app
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
	form.AddCheckbox("Squash pr?", true, func(checked bool) {
		ui.squash = checked
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
		}

		return ev
	})

	go func() {
		if err := app.SetRoot(form, true).EnableMouse(true).Run(); err != nil {
			ui.execErrHandler(err)
		}
		ui.execCancelHandler()
	}()

	return &ui, nil
}

func (ui *tviewUI) execErrHandler(err error) {
	handler := ui.errHandler
	if handler != nil {
		handler(err)
	}
}

func (ui *tviewUI) Stop() {
	ui.app.Stop()
}

func (ui *tviewUI) execCancelHandler() {
	ui.Stop()
	handler := ui.cancelHandler
	if handler != nil {
		handler()
	}
}

func (ui *tviewUI) execCreateHandler() {
	handler := ui.createHandler
	if handler != nil {
		result := UserSelections{
			SourceBranch: &ui.sourceBranch,
			TargetBranch: &ui.targetBranch,
			Repository:   ui.repository,
			WorkItems:    ui.workItems,
			MergeMessage: ui.mergeMessage,
			Squash:       ui.squash,
		}
		handler(result)
	}
}

func (ui *tviewUI) SetRepositories(repositories []string) {
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

func (ui *tviewUI) SetRepository(repository string) {
	i := slices.Index(ui.repositories, repository)
	if i >= 0 {
		ui.repositoriesDropDown.SetCurrentOption(i)
		ui.app.Draw()
	}
}

func (ui *tviewUI) execRepositoryChangeHandler(repository string) {
	handler := ui.repositoryChangeHandler
	if handler != nil {
		handler(repository)
	}
}

func (ui *tviewUI) SetSourceBranches(branches []git.GitBranchStats) {
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

func (ui *tviewUI) SetTargetBranches(branches []git.GitBranchStats) {
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

func (ui *tviewUI) SetWorkItems(workItemIDs []string) {
	initialValue := strings.Join(workItemIDs, ", ")
	ui.workItemsInput.SetText(initialValue)
}

func (ui *tviewUI) SetMergeMessage(mergeMessage string) {
	ui.mergeMessageInput.SetText(mergeMessage, true)
}

func getBranchNames(branches []git.GitBranchStats) []string {
	options := make([]string, 0, len(branches))
	for i := 0; i < len(branches); i++ {
		options = append(options, *branches[i].Name)
	}
	return options
}

func (ui *tviewUI) SetRepositoryChangeHandler(handler func(repository string)) {
	ui.repositoryChangeHandler = handler
}

func (ui *tviewUI) SetTargetBranchChangeHandler(handler func(targetBranch git.GitBranchStats)) {
	ui.targetBranchChangeHandler = handler
}

func (ui *tviewUI) SetSourceBranchChangeHandler(handler func(sourceBranch git.GitBranchStats)) {
	ui.sourceBranchChangeHandler = handler
}

func (ui *tviewUI) SetCreateHandler(handler func(selections UserSelections)) {
	ui.createHandler = handler
}

func (ui *tviewUI) SetCancelHandler(handler func()) {
	ui.cancelHandler = handler
}

func (ui *tviewUI) SetErrHandler(handler func(error)) {
	ui.errHandler = handler
}
