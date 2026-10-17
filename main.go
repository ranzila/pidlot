// pidlot is a full-screen Windows process manager: a live process table
// with a command prompt underneath.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const refreshInterval = 2 * time.Second

// UI bundles the tview application and shared state.
type UI struct {
	app       *tview.Application
	table     *tview.Table
	status    *tview.TextView
	input     *tview.InputField
	state     *State
	col       *collector
	history   *cmdHistory
	refreshCh chan struct{}
}

func main() {
	// Non-TUI mode: `pidlot -once` prints the current process table to stdout.
	if len(os.Args) > 1 && os.Args[1] == "-once" {
		snap := newCollector().snapshot()
		sortProcs(snap)
		fmt.Println("pidlot snapshot:")
		if _, err := exportRows(snap, "txt", "-"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	ui := &UI{
		state:     newState(),
		col:       newCollector(),
		history:   newHistory(),
		refreshCh: make(chan struct{}, 1),
	}

	ui.table = tview.NewTable()
	ui.table.SetFixed(1, 0).
		SetSelectable(false, false)
	ui.table.SetBorder(true)
	ui.table.SetTitle(" pidlot - process manager ")
	ui.table.SetTitleColor(tcell.ColorYellow)

	ui.status = tview.NewTextView().
		SetDynamicColors(true).
		SetText("[darkgray]collecting process data...[-:-:-]")

	ui.input = tview.NewInputField().
		SetLabel(" cmd> ").
		SetFieldWidth(0).
		SetPlaceholder("type a command - 'help' for list")

	ui.input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		cmd := strings.TrimSpace(ui.input.GetText())
		ui.input.SetText("")
		if cmd == "" {
			return
		}
		ui.history.add(cmd)
		msg, isErr, quit := ui.execute(cmd)
		if quit {
			ui.app.Stop()
			return
		}
		ui.state.setMessage(msg, isErr)
		ui.redraw()
	})

	// Ctrl+P / Ctrl+N walk the command history like a shell.
	ui.input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyCtrlP:
			if c, ok := ui.history.prev(); ok {
				ui.input.SetText(c)
			}
			return nil
		case tcell.KeyCtrlN:
			if c, ok := ui.history.next(); ok {
				ui.input.SetText(c)
			} else {
				ui.input.SetText("")
			}
			return nil
		}
		return ev
	})

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.table, 0, 1, false).
		AddItem(ui.status, 2, 1, false).
		AddItem(ui.input, 1, 1, true)

	ui.app = tview.NewApplication().
		SetRoot(root, true).
		SetInputCapture(ui.globalKeys)

	// First snapshot synchronously so the initial frame has data.
	ui.state.setSnapshot(ui.col.snapshot())
	ui.state.setMessage("welcome to pidlot - type a command and press Enter", false)
	ui.redraw()

	go ui.refreshLoop()

	if err := ui.app.Run(); err != nil {
		panic(err)
	}
}

// globalKeys handles keys that must work regardless of focus:
// scrolling the table and app-level shortcuts.
func (ui *UI) globalKeys(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyCtrlC:
		ui.app.Stop()
		return nil
	case tcell.KeyCtrlL:
		ui.requestRefresh()
		return nil
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn,
		tcell.KeyHome, tcell.KeyEnd:
		ui.scrollTable(ev.Key())
		return nil
	}
	return ev
}

func (ui *UI) requestRefresh() {
	select {
	case ui.refreshCh <- struct{}{}:
	default:
	}
}

func (ui *UI) refreshLoop() {
	for {
		select {
		case <-ui.refreshCh:
		case <-time.After(refreshInterval):
		}
		ui.state.setSnapshot(ui.col.snapshot())
		ui.app.QueueUpdateDraw(func() { ui.redraw() })
	}
}

// scrollTable moves the table viewport (border + header consume 3 rows).
func (ui *UI) scrollTable(key tcell.Key) {
	t := ui.table
	rows := t.GetRowCount() // includes header
	_, _, _, height := t.GetRect()
	visible := height - 3 // top+bottom border + header row
	if visible < 1 {
		visible = 1
	}
	row, _ := t.GetOffset()
	switch key {
	case tcell.KeyUp:
		row--
	case tcell.KeyDown:
		row++
	case tcell.KeyPgUp:
		row -= visible
	case tcell.KeyPgDn:
		row += visible
	case tcell.KeyHome:
		row = 0
	case tcell.KeyEnd:
		row = rows
	}
	if row < 0 {
		row = 0
	}
	if max := rows - 1; row > max {
		row = max
	}
	t.SetOffset(row, 0)
}

// --- command history ------------------------------------------------------

type cmdHistory struct {
	items []string
	pos   int // -1 = at the prompt (not browsing)
}

func newHistory() *cmdHistory { return &cmdHistory{pos: -1} }

func (h *cmdHistory) add(s string) {
	h.items = append(h.items, s)
	h.pos = -1
}

func (h *cmdHistory) prev() (string, bool) {
	if len(h.items) == 0 {
		return "", false
	}
	if h.pos == -1 {
		h.pos = len(h.items) - 1
	} else if h.pos > 0 {
		h.pos--
	}
	return h.items[h.pos], true
}

func (h *cmdHistory) next() (string, bool) {
	if h.pos == -1 || h.pos >= len(h.items)-1 {
		h.pos = -1
		return "", false
	}
	h.pos++
	return h.items[h.pos], true
}
