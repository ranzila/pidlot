package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// collector gathers process snapshots and computes CPU% between samples.
type collector struct {
	prevCPU map[int32]float64 // last observed CPU seconds per pid
	prevAt  time.Time         // time of the previous sample
}

func newCollector() *collector {
	return &collector{prevCPU: make(map[int32]float64)}
}

// snapshot collects info for every running process.
func (c *collector) snapshot() []ProcInfo {
	now := time.Now()
	procs, err := process.Processes()
	if err != nil {
		return nil
	}
	ports := listeningPortsByPID()

	infos := make([]ProcInfo, 0, len(procs))
	for _, p := range procs {
		pi := ProcInfo{PID: p.Pid, Name: strconv.Itoa(int(p.Pid))}

		if name, err := p.Name(); err == nil && name != "" {
			pi.Name = name
		}
		if pp, err := p.Ppid(); err == nil {
			pi.PPID = pp
		}
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			pi.RAM = mi.RSS
		}
		if exe, err := p.Exe(); err == nil && exe != "" {
			pi.Path = exe
		}
		if pi.Path == "" {
			pi.Path = pi.Name
		}
		pi.Ports = ports[p.Pid]

		// CPU%: delta since the previous sample; on first sight fall
		// back to the average since process creation.
		if ct, err := p.Times(); err == nil {
			total := ct.Total()
			if prev, ok := c.prevCPU[p.Pid]; ok && !c.prevAt.IsZero() {
				dt := now.Sub(c.prevAt).Seconds()
				if dt > 0.25 {
					pi.CPU = (total - prev) / dt * 100
				}
			} else if cr, err := p.CreateTime(); err == nil && cr > 0 {
				age := now.Sub(time.UnixMilli(cr)).Seconds()
				if age > 1 {
					pi.CPU = total / age * 100
				}
			}
			c.prevCPU[p.Pid] = total
		}
		infos = append(infos, pi)
	}
	c.prevAt = now

	// Drop CPU history of processes that no longer exist.
	alive := make(map[int32]float64, len(infos))
	for _, pi := range infos {
		alive[pi.PID] = c.prevCPU[pi.PID]
	}
	c.prevCPU = alive
	return infos
}

// listeningPortsByPID maps each pid to its listening TCP / bound UDP ports.
func listeningPortsByPID() map[int32][]int {
	sets := make(map[int32]map[int]bool)
	conns, err := gnet.Connections("inet")
	if err != nil {
		return map[int32][]int{}
	}
	for _, cn := range conns {
		if cn.Pid <= 0 || cn.Laddr.Port <= 0 {
			continue
		}
		const sockDgram = 2 // UDP
		if cn.Status == "LISTEN" || cn.Type == sockDgram {
			set, ok := sets[cn.Pid]
			if !ok {
				set = make(map[int]bool)
				sets[cn.Pid] = set
			}
			set[int(cn.Laddr.Port)] = true
		}
	}
	out := make(map[int32][]int, len(sets))
	for pid, set := range sets {
		ports := make([]int, 0, len(set))
		for p := range set {
			ports = append(ports, p)
		}
		sort.Ints(ports)
		out[pid] = ports
	}
	return out
}

// --- small shared helpers -----------------------------------------------

func lower(s string) string { return strings.ToLower(s) }

func containsFold(s, sub string) bool {
	return strings.Contains(lower(s), sub)
}

func contains(pid int32, sub string) bool {
	return strings.Contains(strconv.Itoa(int(pid)), sub)
}

func portIn(ports []int, want int) bool {
	for _, p := range ports {
		if p == want {
			return true
		}
	}
	return false
}

func sortProcs(ps []ProcInfo) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].CPU != ps[j].CPU {
			return ps[i].CPU > ps[j].CPU
		}
		if ps[i].RAM != ps[j].RAM {
			return ps[i].RAM > ps[j].RAM
		}
		return ps[i].PID < ps[j].PID
	})
}
