package tasksui

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/pterm/pterm"
	"github.com/rivo/tview"
)

func (u *ui) editTask(t Task, onSave func(Task), onCancel func()) {
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" " + t.GetTitle() + " ")

	height := pterm.GetTerminalHeight() / 3
	width := pterm.GetTerminalWidth() / 2

	form.AddInputField("Title", t.GetTitle(), width,
		func(textToCheck string, lastChar rune) bool {
			return len(textToCheck) > 0
		},
		func(text string) {
			t.SetTitle(text)
		})

	form.AddTextArea("Description", t.GetDescription(), width, height, 0,
		func(text string) {
			t.SetDescription(text)
		})

	form.AddInputField("Estimate", fmt.Sprintf("%v", t.GetEstimate()), 10,
		func(textToCheck string, lastChar rune) bool {
			estimate, err := strconv.ParseFloat(textToCheck, 32)
			return err == nil && estimate > 0
		},
		func(text string) {
			estimate, _ := strconv.ParseFloat(text, 32)
			t.SetEstimate(float32(estimate))
		})

	form.AddInputField("Tags", t.GetTagsString(), width,
		func(textToCheck string, lastChar rune) bool { return true },
		func(text string) {
			t.SetTagsString(text)
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
