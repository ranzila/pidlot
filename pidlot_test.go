package main

import (
	"strings"
	"testing"
)

func testSnapshot() []ProcInfo {
	return []ProcInfo{
		{PID: 1, PPID: 0, Name: "init.exe", Path: `C:\init.exe`, CPU: 0.5, RAM: 10 << 20, Ports: []int{80}},
		{PID: 2, PPID: 1, Name: "svc.exe", Path: `C:\svc.exe`, CPU: 12.5, RAM: 200 << 20, Ports: []int{443, 8080}},
		{PID: 3, PPID: 1, Name: "chrome.exe", Path: `C:\chrome.exe`, CPU: 2.0, RAM: 1 << 30, Ports: nil},
		{PID: 4, PPID: 2, Name: "worker.exe", Path: `C:\worker.exe`, CPU: 30, RAM: 5 << 20},
	}
}

func TestFilterByText(t *testing.T) {
	s := newState()
	s.setSnapshot(testSnapshot())
	s.mu.Lock()
	s.filterText = "chrome"
	got := s.displayedLocked()
	s.mu.Unlock()
	if len(got) != 1 || got[0].PID != 3 {
		t.Fatalf("filter by text: got %+v", got)
	}
}

func TestFilterByPort(t *testing.T) {
	s := newState()
	s.setSnapshot(testSnapshot())
	s.mu.Lock()
	s.filterPort = 443
	got := s.displayedLocked()
	s.mu.Unlock()
	if len(got) != 1 || got[0].PID != 2 {
		t.Fatalf("filter by port: got %+v", got)
	}
}

func TestChildView(t *testing.T) {
	s := newState()
	s.setSnapshot(testSnapshot())
	s.mu.Lock()
	s.childSet, s.childOf = true, 1
	got := s.displayedLocked()
	s.mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("child view: got %+v", got)
	}
	for _, p := range got {
		if p.PPID != 1 {
			t.Fatalf("child view: unexpected pid %d", p.PID)
		}
	}
}

func TestSortByCPU(t *testing.T) {
	s := newState()
	s.setSnapshot(testSnapshot())
	s.mu.Lock()
	got := s.displayedLocked()
	s.mu.Unlock()
	if got[0].PID != 4 || got[1].PID != 2 {
		t.Fatalf("sort by cpu: got %+v", got)
	}
}

func TestExportFormats(t *testing.T) {
	rows := testSnapshot()
	for _, format := range []string{"txt", "csv", "md"} {
		name, err := exportRows(rows, format, "-")
		if err != nil {
			t.Fatalf("export %s: %v", format, err)
		}
		if name != "stdout" {
			t.Fatalf("unexpected name %q", name)
		}
	}
	// csv correctness
	name, err := exportRows(rows, "csv", "-")
	if err != nil || name != "stdout" {
		t.Fatal(err)
	}
}

func TestPortsText(t *testing.T) {
	if got := portsText(nil); got != "-" {
		t.Fatalf("empty ports: %q", got)
	}
	many := []int{1, 2, 3, 4, 5, 6, 7, 8}
	if got := portsText(many); got != "1,2,3,4,5,6,+2 more" {
		t.Fatalf("many ports: %q", got)
	}
	if !strings.Contains(portsText([]int{80}), "80") {
		t.Fatal("single port")
	}
}
