// Package hw turns hardware facts into a parallelism recommendation for the
// planner (specs/WORKFLOW.md §2.1, `loops hw`). Pure: facts come from the
// hwprobe adapter.
package hw

import (
	"fmt"
	"strconv"
)

// Facts describes the machine. Zero values mean "unknown".
type Facts struct {
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	LogicalCPUs   int      `json:"logical_cpus"`
	CPUQuota      float64  `json:"cpu_quota,omitempty"` // cgroup CPU limit in cores (0 = none)
	GOMAXPROCS    int      `json:"gomaxprocs"`
	TotalRAMBytes uint64   `json:"total_ram_bytes,omitempty"`
	AvailRAMBytes uint64   `json:"avail_ram_bytes,omitempty"`
	MemLimitBytes uint64   `json:"mem_limit_bytes,omitempty"` // cgroup memory limit (0 = none)
	Load1         float64  `json:"load1,omitempty"`
	DiskFreeBytes uint64   `json:"disk_free_bytes,omitempty"`
	GoVersion     string   `json:"go_version,omitempty"`
	Container     bool     `json:"container,omitempty"`
	Notes         []string `json:"notes,omitempty"` // probes that failed or were skipped
}

// Knobs tune the recommendation (`.loops/config.json` "parallel").
type Knobs struct {
	MaxAgents       int     `json:"max_agents,omitempty"`         // default cap, 100 when 0
	AgentRAMGiB     float64 `json:"agent_ram_gib,omitempty"`      // per-agent RAM budget, 1 when 0
	CPUSlotsPerCore int     `json:"cpu_slots_per_core,omitempty"` // agents per logical CPU, 4 when 0
	MaxTestRuns     int     `json:"max_test_runs,omitempty"`      // concurrent go test runs, logicalCPUs/2 when 0
}

// DefaultMaxAgents is the default cap on concurrent agents.
const DefaultMaxAgents = 100

// Recommendation is what the planner consumes.
type Recommendation struct {
	MaxParallelAgents     int    `json:"max_parallel_agents"`
	MaxConcurrentTestRuns int    `json:"max_concurrent_test_runs"`
	GOFLAGS               string `json:"goflags"`
	BoundBy               string `json:"bound_by"` // "cpu" | "ram" | "config" | "unknown"
	Reason                string `json:"reason"`   // one line, e.g. "RAM-bound: 11.2 GiB available → 8 agents"
}

// Formula constants, in one place so they can be tuned together.
const (
	defaultAgentRAMGiB   = 1.0
	defaultSlotsPerCore  = 4
	ramUsableFraction    = 0.75 // share of RAM agents may use
	unknownAvailFraction = 0.5  // of total RAM, when available is unknown
	goflags              = "-p=2"
)

const bytesPerGiB = float64(1 << 30)

// Recommend computes the recommendation from facts and knobs. Pure.
func Recommend(f Facts, k Knobs) Recommendation {
	maxAgents := k.MaxAgents
	if maxAgents <= 0 {
		maxAgents = DefaultMaxAgents
	}
	agentGiB := k.AgentRAMGiB
	if agentGiB <= 0 {
		agentGiB = defaultAgentRAMGiB
	}
	slotsPerCore := k.CPUSlotsPerCore
	if slotsPerCore <= 0 {
		slotsPerCore = defaultSlotsPerCore
	}

	logical := f.LogicalCPUs
	if logical <= 0 {
		logical = f.GOMAXPROCS
	}
	cpus := float64(logical)
	if f.CPUQuota > 0 {
		cpus = f.CPUQuota
	}
	cpuSlots := int(cpus * float64(slotsPerCore))
	if f.CPUQuota > 0 {
		cpuSlots = max(1, cpuSlots)
	}

	ram := f.AvailRAMBytes
	if ram == 0 && f.TotalRAMBytes > 0 {
		ram = uint64(float64(f.TotalRAMBytes) * unknownAvailFraction)
	}
	if f.MemLimitBytes > 0 && (ram == 0 || f.MemLimitBytes < ram) {
		ram = f.MemLimitBytes
	}
	ramGiB := float64(ram) / bytesPerGiB
	ramSlots := int(ramGiB*ramUsableFraction/agentGiB + 1e-9)

	testRuns := k.MaxTestRuns
	if testRuns <= 0 {
		testRuns = max(1, int(cpus/2))
	}
	rec := Recommendation{MaxConcurrentTestRuns: testRuns, GOFLAGS: goflags}

	cpuKnown, ramKnown := cpuSlots > 0, ram > 0
	if !cpuKnown && !ramKnown {
		rec.MaxParallelAgents, rec.BoundBy = 1, "unknown"
		rec.Reason = "unknown: CPU and RAM could not be measured → 1 agent"
		return rec
	}
	slots, bound := ramSlots, "ram"
	if cpuKnown && (!ramKnown || cpuSlots <= ramSlots) {
		slots, bound = cpuSlots, "cpu"
	}
	if slots >= maxAgents {
		bound = "config"
	}
	rec.MaxParallelAgents = min(max(slots, 1), maxAgents)
	rec.BoundBy = bound
	switch bound {
	case "cpu":
		rec.Reason = fmt.Sprintf("CPU-bound: %s cores × %d slots → %d agents (cap %d)", trimFloat(cpus), slotsPerCore, rec.MaxParallelAgents, maxAgents)
	case "ram":
		rec.Reason = fmt.Sprintf("RAM-bound: %.1f GiB available → %d agents (cap %d)", ramGiB, rec.MaxParallelAgents, maxAgents)
	default:
		rec.Reason = fmt.Sprintf("config cap: hardware allows %d, capped at %d agents", slots, maxAgents)
	}
	return rec
}

func trimFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
