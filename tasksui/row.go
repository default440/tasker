package tasksui

import (
	"fmt"
	"tasker/wiki"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type uiRow struct {
	table            *tview.Table
	task             *wiki.Task
	rowNumber        int
	titleWidth       int
	descriptionWidth int
}

func (r *uiRow) draw() {
	tfsTaskID := ""
	switch {
	case r.task.TfsTaskID < 0:
		tfsTaskID = "n/a"
	case r.task.TfsTaskID == 0:
	default:
		tfsTaskID = fmt.Sprintf("%d", r.task.TfsTaskID)
	}

	r.table.SetCell(r.rowNumber, 0, tview.NewTableCell(fmt.Sprintf("%d", r.rowNumber)).SetTextColor(tcell.ColorDimGray))
	r.table.SetCell(r.rowNumber, 1, tview.NewTableCell(cutString(r.task.Title, r.titleWidth, true)))
	r.table.SetCell(r.rowNumber, 2, tview.NewTableCell(cutString(r.task.Description, r.descriptionWidth, true)))
	r.table.SetCell(r.rowNumber, 3, tview.NewTableCell(fmt.Sprintf("%v", r.task.Estimate)))
	r.table.SetCell(r.rowNumber, 4, tview.NewTableCell(tfsTaskID))
}

func newRow(table *tview.Table, task *wiki.Task, rowNumber, titleWidth, descriptionWidth int) *uiRow {
	r := uiRow{
		table:            table,
		task:             task,
		rowNumber:        rowNumber,
		titleWidth:       titleWidth,
		descriptionWidth: descriptionWidth,
	}

	return &r
}
