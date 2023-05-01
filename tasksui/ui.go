package tasksui

import (
	"fmt"
	"strings"
	"tasker/wiki"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/samber/lo"
	"golang.org/x/exp/slices"
)

type ui struct {
	app              *tview.Application
	pages            *tview.Pages
	modalOpened      bool
	rowSelectionMode bool
	tabbedItems      []tview.Primitive
	totalInfo        *tview.TextView
	tables           []*uiTable
	approved         bool
}

func (u *ui) draw() {
	for _, table := range u.tables {
		table.draw()
	}
	u.drawTotal()
}

func (u *ui) drawTotal() {
	if u.totalInfo == nil {
		return
	}

	var tasksTotalCount int
	var totalEstimate float32

	for _, table := range u.tables {
		tasksTotalCount += len(table.tasks)
		for _, t := range table.tasks {
			totalEstimate += t.Estimate
		}
	}

	u.totalInfo.SetText(fmt.Sprintf("Total tasks: %d, estimate: %v", tasksTotalCount, totalEstimate))
}

func PreviewTasks(tables []*wiki.Table) (bool, error) {
	ui := newUI(tables)
	defer ui.app.Stop()

	ui.draw()

	resultChan := make(chan error)
	go func() {
		resultChan <- ui.app.SetRoot(ui.pages, true).EnableMouse(true).Run()
	}()

	return ui.approved, <-resultChan
}

func newUI(tables []*wiki.Table) *ui {
	u := ui{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
	}

	onCancel := func() {
		u.app.Stop()
	}

	onSave := func() {
		u.approved = true
		u.app.Stop()
	}

	u.app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if !u.modalOpened {
			switch {
			case ev.Key() == tcell.KeyTab || ev.Key() == tcell.KeyBacktab:
				if len(u.tabbedItems) > 0 {
					focusedItem := slices.IndexFunc(u.tabbedItems, func(p tview.Primitive) bool {
						return p.HasFocus()
					})
					if focusedItem >= 0 {
						u.tabbedItems[focusedItem].Blur()
					}
					if ev.Key() == tcell.KeyBacktab {
						if focusedItem == 0 {
							focusedItem = len(u.tabbedItems)
						}
						focusedItem -= 1
					} else {
						focusedItem += 1
					}
					u.app.SetFocus(u.tabbedItems[focusedItem%len(u.tabbedItems)])
				}
			case ev.Key() == tcell.KeyESC:
				if !u.rowSelectionMode {
					u.app.Stop()
				}
			case ev.Key() == tcell.KeyEsc:
				onCancel()
			case ev.Key() == tcell.KeyCtrlS:
				onSave()

			}
		}

		return ev
	})

	topGrid := tview.NewGrid()
	u.pages.AddPage("main", topGrid, true, true)
	topGrid.SetRows(0, 3, 1)

	grid := tview.NewGrid()
	grid.SetBorders(true)
	topGrid.AddItem(grid, 0, 0, 1, 1, 0, 0, false)

	createBtn := tview.NewButton("Create")
	createBtn.SetSelectedFunc(onSave)
	u.tabbedItems = append(u.tabbedItems, createBtn)

	cancelBtn := tview.NewButton("Cancel")
	cancelBtn.SetSelectedFunc(onCancel)
	u.tabbedItems = append(u.tabbedItems, cancelBtn)

	btnsFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(createBtn, 0, 1, false).
			AddItem(nil, 3, 0, false).
			AddItem(cancelBtn, 0, 1, false).
			AddItem(nil, 0, 1, false), 0, 1, false).
		AddItem(nil, 0, 1, false)
	topGrid.AddItem(btnsFlex, 1, 0, 1, 1, 0, 0, false)

	topGrid.AddItem(tview.NewTextView().SetText(" Press Ctrl+S for save, press Ctrl+C or ESC to exit"), 2, 0, 1, 1, 0, 0, false)

	gridRowNumber := 0
	for i, table := range tables {
		ut := u.newTable(table)
		u.tables = append(u.tables, ut)
		grid.AddItem(ut.view, gridRowNumber, 0, 1, 1, 0, 0, i == 0)
		gridRowNumber++
	}

	gridRowsSizes := lo.Map(tables, func(_ *wiki.Table, _ int) int {
		return 0
	})

	if len(tables) > 1 {
		gridRowsSizes = append(gridRowsSizes, 1)
		u.totalInfo = tview.NewTextView()
		u.totalInfo.SetTextAlign(tview.AlignRight)
		grid.AddItem(u.totalInfo, gridRowNumber, 0, 1, 1, 1, 1, false)
	}

	grid.SetRows(gridRowsSizes...)

	return &u
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
