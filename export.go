package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// exportRows writes the given rows to fname (default: processes-<timestamp>.<ext>).
func exportRows(rows []ProcInfo, format, fname string) (string, error) {
	var w io.Writer
	if fname == "-" {
		w = os.Stdout
		fname = "stdout"
	} else {
		if fname == "" {
			fname = fmt.Sprintf("processes-%s.%s", time.Now().Format("20060102-150405"), format)
		}
		f, err := os.Create(fname)
		if err != nil {
			return "", err
		}
		defer f.Close()
		w = f
	}

	switch format {
	case "csv":
		cw := csv.NewWriter(w)
		defer cw.Flush()
		if err := cw.Write([]string{"pid", "ppid", "name", "cpu_percent", "ram_bytes", "ram", "ports", "path"}); err != nil {
			return "", err
		}
		for _, p := range rows {
			rec := []string{
				fmt.Sprint(p.PID),
				fmt.Sprint(p.PPID),
				p.Name,
				fmt.Sprintf("%.2f", p.CPU),
				fmt.Sprint(p.RAM),
				fmtBytes(p.RAM),
				portsCSV(p.Ports),
				p.Path,
			}
			if err := cw.Write(rec); err != nil {
				return "", err
			}
		}
		if err := cw.Error(); err != nil {
			return "", err
		}

	case "md":
		fmt.Fprintln(w, "| PID | PPID | Name | CPU % | RAM | Ports | Path |")
		fmt.Fprintln(w, "|---:|---:|---|---:|---:|---|---|")
		for _, p := range rows {
			fmt.Fprintf(w, "| %d | %d | %s | %.1f | %s | %s | %s |\n",
				p.PID, p.PPID, mdEscape(p.Name), p.CPU,
				fmtBytes(p.RAM), mdEscape(portsText(p.Ports)), mdEscape(p.Path))
		}

	default: // txt
		fmt.Fprintf(w, "%-8s %-8s %-30s %8s %12s %-20s %s\n",
			"PID", "PPID", "NAME", "CPU %", "RAM", "PORTS", "PATH")
		fmt.Fprintln(w, strings.Repeat("-", 120))
		for _, p := range rows {
			fmt.Fprintf(w, "%-8d %-8d %-30s %8.1f %12s %-20s %s\n",
				p.PID, p.PPID, truncEnd(p.Name, 30), p.CPU,
				fmtBytes(p.RAM), truncEnd(portsText(p.Ports), 20), p.Path)
		}
	}
	return fname, nil
}

func portsCSV(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = fmt.Sprint(p)
	}
	return strings.Join(parts, " ")
}

func mdEscape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
