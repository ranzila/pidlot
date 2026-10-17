package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// execute parses and runs a prompt command.
// Returns a status message, whether it was an error, and whether to quit.
func (ui *UI) execute(cmd string) (msg string, isErr bool, quit bool) {
	fields := strings.Fields(cmd)
	verb := strings.ToLower(fields[0])
	args := fields[1:]

	switch verb {
	case "quit", "exit", "q":
		return "", false, true

	case "kill":
		return ui.cmdKill(args)

	case "child":
		return ui.cmdChild(args)

	case "filter", "f":
		return ui.cmdFilter(cmd)

	case "port", "p":
		return ui.cmdPort(args)

	case "export":
		return ui.cmdExport(args)

	case "reset", "r":
		ui.state.resetView()
		return "view reset: showing all processes", false, false

	case "refresh":
		ui.requestRefresh()
		return "refreshing...", false, false

	case "help", "h", "?":
		return helpLine, false, false

	default:
		return fmt.Sprintf("unknown command %q - try 'help'", verb), true, false
	}
}

func (ui *UI) cmdKill(args []string) (string, bool, bool) {
	if len(args) == 0 {
		return "usage: kill <pid> [pid...]", true, false
	}
	var results []string
	anyErr := false
	for _, a := range args {
		pid, err := strconv.ParseInt(a, 10, 32)
		if err != nil {
			results = append(results, fmt.Sprintf("%s: invalid pid", a))
			anyErr = true
			continue
		}
		p, err := process.NewProcess(int32(pid))
		if err == nil {
			err = p.Kill()
		}
		if err != nil {
			results = append(results, fmt.Sprintf("%d: %v", pid, err))
			anyErr = true
		} else {
			results = append(results, fmt.Sprintf("%d: killed", pid))
		}
	}
	return strings.Join(results, ", "), anyErr, false
}

func (ui *UI) cmdChild(args []string) (string, bool, bool) {
	if len(args) != 1 {
		return "usage: child <pid>", true, false
	}
	pid, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil {
		return fmt.Sprintf("invalid pid %q", args[0]), true, false
	}
	s := ui.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.snapshot) == 0 {
		return "no process data yet, try again shortly", true, false
	}
	if !s.hasPIDLocked(int32(pid)) {
		return fmt.Sprintf("no running process with pid %d", pid), true, false
	}
	s.childOf, s.childSet = int32(pid), true
	s.filterText, s.filterPort = "", 0
	n := len(s.displayedLocked())
	if n == 0 {
		return fmt.Sprintf("pid %d has no child processes (use 'reset' to go back)", pid), false, false
	}
	return fmt.Sprintf("showing %d child process(es) of pid %d (use 'reset' to go back)", n, pid), false, false
}

func (ui *UI) cmdFilter(cmd string) (string, bool, bool) {
	verb := strings.Fields(cmd)[0]
	rest := strings.TrimSpace(strings.TrimPrefix(cmd, verb))
	s := ui.state
	if rest == "" {
		s.mu.Lock()
		s.filterText = ""
		s.childSet = false
		s.mu.Unlock()
		return "filter cleared", false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filterText = rest
	s.filterPort = 0
	s.childSet = false
	n := len(s.displayedLocked())
	if n == 0 {
		return fmt.Sprintf("filter %q: no matching process", rest), true, false
	}
	return fmt.Sprintf("filter %q: %d process(es)", rest, n), false, false
}

func (ui *UI) cmdPort(args []string) (string, bool, bool) {
	s := ui.state
	if len(args) == 0 {
		s.mu.Lock()
		s.filterPort = 0
		s.mu.Unlock()
		return "port filter cleared", false, false
	}
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Sprintf("invalid port %q", args[0]), true, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filterPort = port
	s.filterText = ""
	s.childSet = false
	n := len(s.displayedLocked())
	if n == 0 {
		return fmt.Sprintf("port %d: no matching process", port), true, false
	}
	return fmt.Sprintf("port %d: %d process(es)", port, n), false, false
}

func (ui *UI) cmdExport(args []string) (string, bool, bool) {
	if len(args) == 0 || (args[0] != "txt" && args[0] != "csv" && args[0] != "md") {
		return "usage: export txt|csv|md [filename]", true, false
	}
	format := args[0]
	fname := ""
	if len(args) > 1 {
		fname = args[1]
	}

	s := ui.state
	s.mu.Lock()
	rows := s.displayedLocked()
	s.mu.Unlock()

	if len(rows) == 0 {
		return "nothing to export", true, false
	}
	path, err := exportRows(rows, format, fname)
	if err != nil {
		return fmt.Sprintf("export failed: %v", err), true, false
	}
	return fmt.Sprintf("exported %d process(es) to %s", len(rows), path), false, false
}
