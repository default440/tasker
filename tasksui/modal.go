package tasksui

import (
	"github.com/pterm/pterm"
	"github.com/rivo/tview"
)

const (
	pageNameModal = "modal"
)

func (u *ui) openModal(p tview.Primitive) {
	modal := func(p tview.Primitive) tview.Primitive {
		flex := tview.NewFlex()

		height := pterm.GetTerminalHeight()
		width := pterm.GetTerminalWidth()

		height -= height / 3
		width -= width / 4

		return flex.
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	u.pages.AddPage(pageNameModal, modal(p), true, true)
	u.modalOpened = true
}

func (u *ui) closeModal() {
	u.pages.RemovePage(pageNameModal)
	u.modalOpened = false
}
