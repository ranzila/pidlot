package main

import (
	"sync"
	"time"
)

// ProcInfo is one row of the process table.
type ProcInfo struct {
	PID      int32
	PPID     int32
	Name     string
	Path     string
	CPU      float64 // percent of one core (can exceed 100 on multicore)
	RAM      uint64  // resident set size in bytes
	Ports    []int   // listening TCP / bound UDP ports
	Children int     // number of direct child processes
}

// State holds the current snapshot and the active view filters.
type State struct {
	mu         sync.Mutex
	snapshot   []ProcInfo
	filterText string
	filterPort int
	childOf    int32
	childSet   bool
	sortKey    string // "pid", "ppid", "children", "name", "cpu", "ram", "ports", "path"
	sortDir    string // "asc" or "desc"
	message    string
	msgErr     bool
	updated    time.Time
}

func newState() *State { return &State{sortKey: "cpu", sortDir: "desc"} }

func (s *State) setSnapshot(sn []ProcInfo) {
	s.mu.Lock()
	s.snapshot = sn
	s.updated = time.Now()
	s.mu.Unlock()
}

func (s *State) setMessage(msg string, isErr bool) {
	s.mu.Lock()
	s.message = msg
	s.msgErr = isErr
	s.mu.Unlock()
}

func (s *State) resetView() {
	s.mu.Lock()
	s.filterText = ""
	s.filterPort = 0
	s.childSet = false
	s.childOf = 0
	s.mu.Unlock()
}

func (s *State) setSort(key, dir string) {
	s.mu.Lock()
	s.sortKey = key
	s.sortDir = dir
	s.mu.Unlock()
}

func (s *State) hasPIDLocked(pid int32) bool {
	for _, p := range s.snapshot {
		if p.PID == pid {
			return true
		}
	}
	return false
}

// displayedLocked returns the processes matching the current view
// (child view / text filter / port filter), sorted by CPU then RAM.
// Caller must hold s.mu.
func (s *State) displayedLocked() []ProcInfo {
	var out []ProcInfo
	ft := lower(s.filterText)
	for _, p := range s.snapshot {
		if s.childSet {
			if p.PPID != s.childOf {
				continue
			}
		} else if s.filterText != "" || s.filterPort > 0 {
			if s.filterText != "" {
				if !containsFold(p.Name, ft) &&
					!containsFold(p.Path, ft) &&
					!contains(p.PID, ft) {
					continue
				}
			}
			if s.filterPort > 0 && !portIn(p.Ports, s.filterPort) {
				continue
			}
		}
		out = append(out, p)
	}
	sortProcs(out, s.sortKey, s.sortDir)
	return out
}
