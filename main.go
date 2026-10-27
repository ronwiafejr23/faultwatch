// faultwatch: live page-fault monitor for a Linux process.
// Usage: faultwatch <pid> [interval-seconds]
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// sample is one snapshot of a process's memory stats.
type sample struct {
	minflt uint64 // minor faults: page was already in RAM, just needed mapping
	majflt uint64 // major faults: page had to be read from disk/SSD
	rssKB  uint64 // resident set size: pages actually in physical memory
	vszKB  uint64 // virtual size: whole virtual address space
}

// readStat parses /proc/<pid>/stat for the fault counters.
func readStat(pid int) (minflt, majflt uint64, err error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, err
	}
	s := string(data)

	// Field 2 is "(command name)" and can contain spaces, so skip past the last ')'.
	end := strings.LastIndexByte(s, ')')
	if end < 0 {
		return 0, 0, fmt.Errorf("unexpected stat format")
	}
	fields := strings.Fields(s[end+1:])
	// After ')': fields[0]=state(3), ... fields[7]=minflt(10), fields[9]=majflt(12)
	if len(fields) < 10 {
		return 0, 0, fmt.Errorf("stat too short")
	}
	if minflt, err = strconv.ParseUint(fields[7], 10, 64); err != nil {
		return 0, 0, err
	}
	if majflt, err = strconv.ParseUint(fields[9], 10, 64); err != nil {
		return 0, 0, err
	}
	return minflt, majflt, nil
}

// readStatus parses VmRSS and VmSize (in kB) from /proc/<pid>/status.
func readStatus(pid int) (rssKB, vszKB uint64, err error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text()) // e.g. ["VmRSS:", "48232", "kB"]
		if len(parts) < 2 {
			continue
		}
		switch parts[0] {
		case "VmRSS:":
			rssKB, _ = strconv.ParseUint(parts[1], 10, 64)
		case "VmSize:":
			vszKB, _ = strconv.ParseUint(parts[1], 10, 64)
		}
	}
	return rssKB, vszKB, sc.Err()
}

func readSample(pid int) (sample, error) {
	minf, majf, err := readStat(pid)
	if err != nil {
		return sample{}, err
	}
	rss, vsz, err := readStatus(pid)
	if err != nil {
		return sample{}, err
	}
	return sample{minf, majf, rss, vsz}, nil
}

func processName(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: faultwatch <pid> [interval-seconds]")
		os.Exit(1)
	}
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "pid must be a number")
		os.Exit(1)
	}
	interval := 1.0
	if len(os.Args) >= 3 {
		if v, err := strconv.ParseFloat(os.Args[2], 64); err == nil && v > 0 {
			interval = v
		}
	}

	prev, err := readSample(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read process %d: %v\n", pid, err)
		os.Exit(1)
	}

	page := os.Getpagesize()
	fmt.Printf("watching pid %d (%s), page size %d bytes\n", pid, processName(pid), page)
	fmt.Printf("%-9s %10s %10s %9s %9s\n", "TIME", "MINFLT/s", "MAJFLT/s", "RSS(MB)", "VSZ(MB)")

	var totalMin, totalMaj uint64
	for {
		time.Sleep(time.Duration(interval * float64(time.Second)))

		cur, err := readSample(pid)
		if err != nil {
			// The /proc entry vanishes when the process exits.
			fmt.Printf("\nprocess %d exited. faults while watching: %d minor, %d major\n",
				pid, totalMin, totalMaj)
			return
		}

		dMin := cur.minflt - prev.minflt
		dMaj := cur.majflt - prev.majflt
		totalMin += dMin
		totalMaj += dMaj

		marker := ""
		if dMaj > 0 {
			marker = "  <- major faults (disk I/O)"
		} else if dMin > 10000 {
			marker = "  <- minor fault spike"
		}

		fmt.Printf("%-9s %10.0f %10.0f %9.1f %9.1f%s\n",
			time.Now().Format("15:04:05"),
			float64(dMin)/interval, float64(dMaj)/interval,
			float64(cur.rssKB)/1024, float64(cur.vszKB)/1024,
			marker)
		prev = cur
	}
}
