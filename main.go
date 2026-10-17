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
	pages     *tview.Pages
	menuOpen  bool // true while the Ctrl+P modal is on screen
}

// menuItem is one selectable entry of the Ctrl+P modal.
type menuItem struct {
	label string
	run   func()
}

func main() {
	// Non-TUI mode: `pidlot -once` prints the current process table to stdout.
	if len(os.Args) > 1 && os.Args[1] == "-once" {
		snap := newCollector().snapshot()
		sortProcs(snap, "cpu", "desc")
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
		ui.runCommandText(cmd)
	})

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.table, 0, 1, false).
		AddItem(ui.status, 2, 1, false).
		AddItem(ui.input, 1, 1, true)

	ui.pages = tview.NewPages()
	ui.pages.AddPage("main", root, true, true)
	ui.pages.AddPage("menu", center(nil, 0, 0), true, false) // replaced on open

	ui.app = tview.NewApplication().
		SetRoot(ui.pages, true).
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
// scrolling the table, the Ctrl+P modal and app-level shortcuts.
func (ui *UI) globalKeys(ev *tcell.EventKey) *tcell.EventKey {
	// While the modal is open, only Ctrl+C / Ctrl+P close it; everything
	// else goes to the list (navigation, Enter, Esc).
	if ui.menuOpen {
		if ev.Key() == tcell.KeyCtrlC || ev.Key() == tcell.KeyCtrlP {
			ui.closeMenu()
			return nil
		}
		return ev
	}

	switch ev.Key() {
	case tcell.KeyCtrlC:
		ui.app.Stop()
		return nil
	case tcell.KeyCtrlL:
		ui.requestRefresh()
		return nil
	case tcell.KeyCtrlP:
		ui.openCommandMenu()
		return nil
	case tcell.KeyUp, tcell.KeyDown:
		// With text in the prompt, Up/Down walk the command history;
		// otherwise they scroll the process table.
		if txt := ui.input.GetText(); txt != "" {
			if ev.Key() == tcell.KeyUp {
				if c, ok := ui.history.prev(); ok {
					ui.input.SetText(c)
				}
			} else {
				if c, ok := ui.history.next(); ok {
					ui.input.SetText(c)
				} else {
					ui.input.SetText("")
				}
			}
			return nil
		}
		ui.scrollTable(ev.Key())
		return nil
	case tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		ui.scrollTable(ev.Key())
		return nil
	}
	return ev
}

// runCommandText executes a prompt command and reflects the result in the
// status bar (without adding it to the history).
func (ui *UI) runCommandText(cmd string) {
	msg, isErr, quit := ui.execute(cmd)
	if quit {
		ui.app.Stop()
		return
	}
	ui.state.setMessage(msg, isErr)
	ui.redraw()
}

// openCommandMenu shows the Ctrl+P modal: commands that take no arguments,
// plus the sort entry that opens a submenu with the column names.
func (ui *UI) openCommandMenu() {
	items := []menuItem{
		{"refresh  - rescan processes now", func() { ui.runCommandText("refresh") }},
		{"reset    - clear filters, show all processes", func() { ui.runCommandText("reset") }},
		{"help     - show command help", func() { ui.runCommandText("help") }},
		{"sort     - choose the sort column...", func() { ui.openSortMenu() }},
		{"quit     - exit pidlot", func() { ui.app.Stop() }},
	}
	ui.openMenu(" commands (no args) ", items)
}

// openSortMenu shows the column-picker submenu of the modal.
func (ui *UI) openSortMenu() {
	items := []menuItem{
		{`« back`, func() { ui.openCommandMenu() }},
	}
	for _, col := range sortMenuColumns {
		col := col
		items = append(items, menuItem{
			fmt.Sprintf("sort by %-8s (%s by default)", col.label, sortColumns[col.key]),
			func() {
				ui.state.setSort(col.key, sortColumns[col.key])
				ui.state.setMessage(fmt.Sprintf("sorted by %s (%s)", col.key, sortColumns[col.key]), false)
				ui.redraw()
			},
		})
	}
	ui.openMenu(" sort by column ", items)
}

// openMenu shows a centered, bordered list on top of the main view.
func (ui *UI) openMenu(title string, items []menuItem) {
	list := tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	for _, it := range items {
		it := it
		list.AddItem(it.label, "", 0, func() {
			ui.closeMenu()
			it.run()
		})
	}
	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			ui.closeMenu()
			return nil
		}
		return ev
	})
	list.SetBorder(true)
	list.SetTitle(title)
	list.SetTitleColor(tcell.ColorYellow)

	width := 46
	height := len(items) + 2 // items + top/bottom border
	ui.pages.RemovePage("menu")
	ui.pages.AddPage("menu", center(list, width, height), true, true)
	ui.menuOpen = true
	ui.app.SetFocus(list)
}

// closeMenu hides the modal and returns focus to the prompt.
func (ui *UI) closeMenu() {
	if !ui.menuOpen {
		return
	}
	ui.pages.HidePage("menu")
	ui.menuOpen = false
	ui.app.SetFocus(ui.input)
}

// center wraps a primitive so it is drawn centered over the page below.
func center(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
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
