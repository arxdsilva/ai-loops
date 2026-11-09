package hwprobe

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// noLimit is the threshold above which a cgroup value means "unlimited".
const noLimit = uint64(1) << 60

// parseMeminfo reads MemTotal and MemAvailable (bytes) from /proc/meminfo.
func parseMeminfo(s string) (total, avail uint64, err error) {
	for _, line := range strings.Split(s, "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		v, perr := strconv.ParseUint(fields[0], 10, 64)
		if perr != nil {
			continue
		}
		if len(fields) > 1 && strings.EqualFold(fields[1], "kB") {
			v *= 1024
		}
		switch key {
		case "MemTotal":
			total = v
		case "MemAvailable":
			avail = v
		}
	}
	if total == 0 && avail == 0 {
		return 0, 0, errors.New("no MemTotal/MemAvailable found")
	}
	return total, avail, nil
}

// parseLoadavg returns the 1-minute load from /proc/loadavg.
func parseLoadavg(s string) (float64, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, errors.New("empty loadavg")
	}
	return strconv.ParseFloat(f[0], 64)
}

// parseCPUMax parses cgroup v2 cpu.max ("max 100000" or "200000 100000") into
// cores; 0 means no limit.
func parseCPUMax(s string) (float64, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, errors.New("empty cpu.max")
	}
	if f[0] == "max" {
		return 0, nil
	}
	period := "100000"
	if len(f) > 1 {
		period = f[1]
	}
	return quotaCores(f[0], period)
}

// quotaCores converts a cfs quota/period pair (microseconds) to cores. A
// quota <= 0 (v1 uses -1) means no limit.
func quotaCores(quota, period string) (float64, error) {
	q, err := strconv.ParseInt(strings.TrimSpace(quota), 10, 64)
	if err != nil {
		return 0, err
	}
	p, err := strconv.ParseInt(strings.TrimSpace(period), 10, 64)
	if err != nil {
		return 0, err
	}
	if q <= 0 {
		return 0, nil
	}
	if p <= 0 {
		return 0, errors.New("bad cfs period")
	}
	return float64(q) / float64(p), nil
}

// parseMemLimit parses memory.max / memory.limit_in_bytes; "max" and huge
// values mean no limit (0).
func parseMemLimit(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty memory limit")
	}
	if s == "max" {
		return 0, nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v >= noLimit {
		return 0, nil
	}
	return v, nil
}

// parseSysctlUint parses `sysctl -n hw.memsize` output.
func parseSysctlUint(s string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(s), 10, 64)
}

// parseDarwinLoadavg parses `sysctl -n vm.loadavg` ("{ 1.50 1.20 1.10 }").
func parseDarwinLoadavg(s string) (float64, error) {
	s = strings.Trim(strings.TrimSpace(s), "{}")
	return parseLoadavg(s)
}

// parseVMStat returns available bytes (free + inactive + speculative pages
// times the page size) from `vm_stat` output.
func parseVMStat(s string) (uint64, error) {
	page := uint64(0)
	var pages uint64
	found := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			if i := strings.Index(line, "page size of "); i >= 0 {
				fields := strings.Fields(line[i+len("page size of "):])
				if len(fields) > 0 {
					page, _ = strconv.ParseUint(fields[0], 10, 64)
				}
			}
			continue
		}
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Pages free", "Pages inactive", "Pages speculative":
			v, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), "."), 10, 64)
			if err != nil {
				continue
			}
			pages += v
			found++
		}
	}
	if page == 0 || found == 0 {
		return 0, errors.New("unrecognised vm_stat output")
	}
	return pages * page, nil
}

// parseWindowsMem parses the PowerShell JSON (values in KiB) into bytes.
func parseWindowsMem(s string) (total, avail uint64, err error) {
	var v struct {
		Free  uint64 `json:"FreePhysicalMemory"`
		Total uint64 `json:"TotalVisibleMemorySize"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &v); err != nil {
		return 0, 0, fmt.Errorf("powershell json: %w", err)
	}
	if v.Total == 0 && v.Free == 0 {
		return 0, 0, errors.New("powershell json had no memory fields")
	}
	return v.Total * 1024, v.Free * 1024, nil
}

// containerHint reports whether /proc/1/cgroup text suggests a container.
func containerHint(cgroup string) bool {
	for _, k := range []string{"docker", "kubepods", "containerd", "lxc", "podman"} {
		if strings.Contains(cgroup, k) {
			return true
		}
	}
	return false
}
