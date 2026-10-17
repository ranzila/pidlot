package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const helpLine = "kill <pid> | child <pid> | filter <text> | port <n> | export txt|csv|md [file] | reset | refresh | help | quit  --  arrows/PgUp/PgDn scroll, Ctrl+L refresh, Ctrl+C quit"

var tableHeaders = []string{"PID", "PPID", "NAME", "CPU %", "RAM", "PORTS", "PATH"}

// redraw re-renders the table and status bar from the current state.
func (ui *UI) redraw() {
	s := ui.state
	s.mu.Lock()
	defer s.mu.Unlock()
	ui.renderLocked()
}

// renderLocked redraws everything; caller must hold state.mu.
func (ui *UI) renderLocked() {
	s := ui.state
	rows := s.displayedLocked()
	t := ui.table
	t.Clear()

	for c, h := range tableHeaders {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorWhite).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetExpansion(1)
		if c >= 3 && c <= 4 { // CPU % / RAM columns right aligned
			cell.SetAlign(tview.AlignRight)
		}
		t.SetCell(0, c, cell)
	}

	for i, p := range rows {
		r := i + 1
		pid := tview.NewTableCell(fmt.Sprint(p.PID)).
			SetTextColor(tcell.ColorYellow).SetSelectable(false)
		ppid := tview.NewTableCell(fmt.Sprint(p.PPID)).
			SetTextColor(tcell.ColorDarkGray).SetSelectable(false)
		name := tview.NewTableCell(truncEnd(p.Name, 40)).
			SetTextColor(tcell.ColorWhite).SetSelectable(false)
		cpu := tview.NewTableCell(fmt.Sprintf("%.1f", p.CPU)).
			SetTextColor(cpuColor(p.CPU)).SetAlign(tview.AlignRight).SetSelectable(false)
		ram := tview.NewTableCell(fmtBytes(p.RAM)).
			SetTextColor(ramColor(p.RAM)).SetAlign(tview.AlignRight).SetSelectable(false)
		ports := tview.NewTableCell(truncEnd(portsText(p.Ports), 24)).
			SetTextColor(tcell.ColorGreen).SetSelectable(false)
		path := tview.NewTableCell(truncStart(p.Path, 80)).
			SetTextColor(tcell.ColorDarkGray).SetMaxWidth(80).SetSelectable(false)
		t.SetCell(r, 0, pid)
		t.SetCell(r, 1, ppid)
		t.SetCell(r, 2, name)
		t.SetCell(r, 3, cpu)
		t.SetCell(r, 4, ram)
		t.SetCell(r, 5, ports)
		t.SetCell(r, 6, path)
	}

	ui.status.SetText(s.statusLocked(len(rows)))
}

// statusLocked builds the two-line status bar; caller must hold state.mu.
func (s *State) statusLocked(shown int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[yellow::b]%d[-:-:-] of %d processes",
		shown, len(s.snapshot))

	var view []string
	if s.childSet {
		view = append(view, fmt.Sprintf("children of PID %d", s.childOf))
	}
	if s.filterText != "" {
		view = append(view, fmt.Sprintf("filter %q", s.filterText))
	}
	if s.filterPort > 0 {
		view = append(view, fmt.Sprintf("port %d", s.filterPort))
	}
	if len(view) > 0 {
		fmt.Fprintf(&b, "  [aqua::b]view:[-:-:-] %s", strings.Join(view, ", "))
	}
	if !s.updated.IsZero() {
		fmt.Fprintf(&b, "  [darkgray]updated %s[-:-:-]", s.updated.Format("15:04:05"))
	}

	if s.message != "" {
		tag := "green"
		if s.msgErr {
			tag = "red"
		}
		fmt.Fprintf(&b, "\n[%s::b]%s[-:-:-]", tag, s.message)
	} else {
		fmt.Fprintf(&b, "\n[darkgray]%s[-:-:-]", helpLine)
	}
	return b.String()
}

// --- display helpers ------------------------------------------------------

func cpuColor(v float64) tcell.Color {
	switch {
	case v >= 50:
		return tcell.ColorRed
	case v >= 10:
		return tcell.ColorYellow
	default:
		return tcell.ColorWhite
	}
}

func ramColor(bytes uint64) tcell.Color {
	if bytes >= 1<<30 {
		return tcell.ColorYellow
	}
	return tcell.ColorWhite
}

func fmtBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func portsText(ports []int) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for i, p := range ports {
		if i == 6 { // cap at 6 + counter
			parts = append(parts, fmt.Sprintf("+%d more", len(ports)-i))
			break
		}
		parts = append(parts, fmt.Sprint(p))
	}
	return strings.Join(parts, ",")
}

// truncEnd truncates the end of s to at most n runes.
func truncEnd(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// truncStart keeps the end of s (path suffix) to at most n runes.
func truncStart(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-(n-1):])
}
