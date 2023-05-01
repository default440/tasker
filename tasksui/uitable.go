package tasksui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/pterm/pterm"
	"github.com/rivo/tview"
)

type uiTable struct {
	tasks      []Task
	view       *tview.Table
	rows       []*uiRow
	headerRows int
}

func (ut *uiTable) draw() {
	for i := 0; i < len(ut.rows); i++ {
		ut.rows[i].draw()
	}
}

func (u *ui) newTable(table Table) *uiTable {
	view := tview.NewTable()
	tasks := table.GetTasks()

	ut := uiTable{
		tasks: tasks,
		view:  view,
	}

	editRow := func(rowNumber int) {
		taskIndex := rowNumber - ut.headerRows
		if taskIndex < 0 || taskIndex >= len(tasks) {
			return
		}
		u.editTask(tasks[taskIndex].Clone(),
			func(updatedTask Task) {
				row := ut.rows[taskIndex]
				row.task = updatedTask
				table.SetTask(updatedTask, taskIndex)
				row.draw()
				u.drawTotal()
				u.app.SetFocus(view)
			},
			func() {
				u.app.SetFocus(view)
			})
	}

	view.
		Select(0, 0).
		SetFixed(1, 1).
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEscape {
				view.SetSelectable(false, false)
				u.rowSelectionMode = false
			}
			if key == tcell.KeyEnter {
				view.SetSelectable(true, false)
				u.rowSelectionMode = true
			}
		}).
		SetSelectedFunc(func(row int, column int) {
			if row != 0 && row <= len(tasks) {
				editRow(row)
			}
		}).
		SetFocusFunc(func() {
			view.SetSelectable(true, false)
			u.rowSelectionMode = true
		}).
		SetBlurFunc(func() {
			view.SetSelectable(false, false)
			u.rowSelectionMode = false
		}).
		SetMouseCapture(func(act tview.MouseAction, ev *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			switch act {
			case tview.MouseLeftDoubleClick:
				row, _ := view.GetSelection()
				if row >= 0 {
					editRow(row)
				}
			}

			return act, ev
		})

	u.tabbedItems = append(u.tabbedItems, view)

	titleWidth, descriptionWidth := getColumnsWidth()
	ut.createRows(titleWidth, descriptionWidth)

	return &ut
}

func (ut *uiTable) createRows(titleWidth, descriptionWidth int) {

	var totalEstimate float32
	headers := []string{"#", "Title", "Description", "Estimate", "TFS"}

	row := 0

	for i, header := range headers {
		ut.view.SetCell(row, i, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignCenter))
	}
	row++
	ut.headerRows = row

	for _, task := range ut.tasks {
		totalEstimate += task.GetEstimate()
		ut.rows = append(ut.rows, newRow(ut.view, task, row, titleWidth, descriptionWidth))
		row++
	}

	footers := []string{"", "∑", "", fmt.Sprintf("%v", totalEstimate), ""}
	for i, footer := range footers {
		ut.view.SetCell(row, i, tview.NewTableCell(footer).SetTextColor(tcell.ColorYellow))
	}
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
