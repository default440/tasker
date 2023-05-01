package tasksui

import (
	"fmt"
	"strconv"
	"tasker/wiki"

	"github.com/gdamore/tcell/v2"
	"github.com/pterm/pterm"
	"github.com/rivo/tview"
)

func (u *ui) editTask(t wiki.Task, onSave func(wiki.Task), onCancel func()) {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" " + t.Title + " ")

	height := pterm.GetTerminalHeight() / 3
	width := pterm.GetTerminalWidth() / 2

	form.AddInputField("Title", t.Title, width,
		func(textToCheck string, lastChar rune) bool {
			return len(textToCheck) > 0
		},
		func(text string) {
			t.Title = text
		})

	form.AddTextArea("Description", t.Description, width, height, 0,
		func(text string) {
			t.Description = text
		})

	form.AddInputField("Estimate", fmt.Sprintf("%v", t.Estimate), 10,
		func(textToCheck string, lastChar rune) bool {
			estimate, err := strconv.ParseFloat(textToCheck, 32)
			return err == nil && estimate > 0
		},
		func(text string) {
			estimate, _ := strconv.ParseFloat(text, 32)
			t.Estimate = float32(estimate)
		})

	saveCb := func() {
		u.closeModal()
		if onSave != nil {
			onSave(t)
		}
	}

	cancelCb := func() {
		u.closeModal()
		if onCancel != nil {
			onCancel()
		}
	}

	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEsc:
			cancelCb()
		case ev.Key() == tcell.KeyCtrlS:
			saveCb()
		}

		return ev
	})

	form.AddButton("Save", saveCb)
	form.AddButton("Cancel", cancelCb)

	u.openModal(form)
}
