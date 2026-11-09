package hwprobe

import (
	"os"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

// probeLinuxAt reads the Linux sources under the given roots (injectable for
// tests).
func probeLinuxAt(f *hw.Facts, proc, cgroup, dockerenv string) {
	if b, err := os.ReadFile(proc + "/meminfo"); err != nil {
		f.Notes = append(f.Notes, "ram: "+err.Error())
	} else if t, a, err := parseMeminfo(string(b)); err != nil {
		f.Notes = append(f.Notes, "ram: "+err.Error())
	} else {
		f.TotalRAMBytes, f.AvailRAMBytes = t, a
	}
	if b, err := os.ReadFile(proc + "/loadavg"); err != nil {
		f.Notes = append(f.Notes, "load: "+err.Error())
	} else if l, err := parseLoadavg(string(b)); err != nil {
		f.Notes = append(f.Notes, "load: "+err.Error())
	} else {
		f.Load1 = l
	}
	probeCgroup(f, cgroup)
	if _, err := os.Stat(dockerenv); err == nil {
		f.Container = true
	} else if b, err := os.ReadFile(proc + "/1/cgroup"); err == nil && containerHint(string(b)) {
		f.Container = true
	}
}

func readTrim(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func probeCgroup(f *hw.Facts, root string) {
	if s, ok := readTrim(root + "/cpu.max"); ok { // v2
		if c, err := parseCPUMax(s); err == nil {
			f.CPUQuota = c
		} else {
			f.Notes = append(f.Notes, "cgroup cpu: "+err.Error())
		}
	} else if q, ok := readTrim(root + "/cpu/cpu.cfs_quota_us"); ok { // v1
		p, _ := readTrim(root + "/cpu/cpu.cfs_period_us")
		if p == "" {
			p = "100000"
		}
		if c, err := quotaCores(q, p); err == nil {
			f.CPUQuota = c
		} else {
			f.Notes = append(f.Notes, "cgroup cpu: "+err.Error())
		}
	}
	mem, ok := readTrim(root + "/memory.max")
	if !ok {
		mem, ok = readTrim(root + "/memory/memory.limit_in_bytes")
	}
	if ok {
		if v, err := parseMemLimit(mem); err == nil {
			f.MemLimitBytes = v
		} else {
			f.Notes = append(f.Notes, "cgroup memory: "+err.Error())
		}
	}
}
