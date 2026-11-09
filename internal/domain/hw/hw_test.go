package hw

import (
	"strings"
	"testing"
)

const gib = uint64(1) << 30

func TestRecommend(t *testing.T) {
	tests := []struct {
		name      string
		f         Facts
		k         Knobs
		agents    int
		tests     int
		bound     string
		reasonHas string
	}{
		{"ram bound", Facts{LogicalCPUs: 16, AvailRAMBytes: 11*gib + gib/5}, Knobs{}, 8, 8, "ram", "RAM-bound: 11.2 GiB available → 8 agents (cap 100)"},
		{"cpu bound", Facts{LogicalCPUs: 2, AvailRAMBytes: 64 * gib}, Knobs{}, 8, 1, "cpu", "CPU-bound"},
		{"config cap binds", Facts{LogicalCPUs: 64, AvailRAMBytes: 256 * gib}, Knobs{}, 100, 32, "config", "config cap"},
		{"config cap custom", Facts{LogicalCPUs: 64, AvailRAMBytes: 256 * gib}, Knobs{MaxAgents: 10}, 10, 32, "config", "capped at 10"},
		{"all unknown", Facts{}, Knobs{}, 1, 1, "unknown", "unknown"},
		{"gomaxprocs fallback", Facts{GOMAXPROCS: 4}, Knobs{}, 16, 2, "cpu", "CPU-bound"},
		{"cpu quota", Facts{LogicalCPUs: 64, CPUQuota: 2, AvailRAMBytes: 64 * gib}, Knobs{}, 8, 1, "cpu", "CPU-bound"},
		{"quota 0.1", Facts{LogicalCPUs: 16, CPUQuota: 0.1, AvailRAMBytes: 64 * gib}, Knobs{}, 1, 1, "cpu", "CPU-bound"},
		{"quota 0.5", Facts{LogicalCPUs: 16, CPUQuota: 0.5, AvailRAMBytes: 64 * gib}, Knobs{}, 2, 1, "cpu", "CPU-bound"},
		{"quota 1.5", Facts{LogicalCPUs: 16, CPUQuota: 1.5, AvailRAMBytes: 64 * gib}, Knobs{}, 6, 1, "cpu", "CPU-bound"},
		{"mem limit below avail", Facts{LogicalCPUs: 64, AvailRAMBytes: 64 * gib, MemLimitBytes: 4 * gib}, Knobs{}, 3, 32, "ram", "RAM-bound"},
		{"mem limit above avail", Facts{LogicalCPUs: 64, AvailRAMBytes: 8 * gib, MemLimitBytes: 64 * gib}, Knobs{}, 6, 32, "ram", "RAM-bound"},
		{"avail unknown uses half total", Facts{LogicalCPUs: 64, TotalRAMBytes: 16 * gib}, Knobs{}, 6, 32, "ram", "RAM-bound"},
		{"ram only known", Facts{AvailRAMBytes: 8 * gib}, Knobs{}, 6, 1, "ram", "RAM-bound"},
		{"tiny ram clamps to 1", Facts{LogicalCPUs: 8, AvailRAMBytes: gib / 2}, Knobs{}, 1, 4, "ram", "RAM-bound"},
		{"agent ram knob", Facts{LogicalCPUs: 64, AvailRAMBytes: 16 * gib}, Knobs{AgentRAMGiB: 2}, 6, 32, "ram", "RAM-bound"},
		{"slots per core knob", Facts{LogicalCPUs: 4, AvailRAMBytes: 64 * gib}, Knobs{CPUSlotsPerCore: 1}, 4, 2, "cpu", "CPU-bound"},
		{"max test runs knob", Facts{LogicalCPUs: 16, AvailRAMBytes: 64 * gib}, Knobs{MaxTestRuns: 3}, 48, 3, "ram", "RAM-bound"},
		{"single cpu tests >=1", Facts{LogicalCPUs: 1, AvailRAMBytes: 64 * gib}, Knobs{}, 4, 1, "cpu", "CPU-bound"},
		{"tie cpu==ram is cpu", Facts{LogicalCPUs: 2, AvailRAMBytes: gib * 32 / 3}, Knobs{}, 8, 1, "cpu", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Recommend(tc.f, tc.k)
			if r.MaxParallelAgents != tc.agents || r.MaxConcurrentTestRuns != tc.tests || r.BoundBy != tc.bound {
				t.Fatalf("got %+v", r)
			}
			if r.GOFLAGS != "-p=2" {
				t.Errorf("goflags %q", r.GOFLAGS)
			}
			if !strings.Contains(r.Reason, tc.reasonHas) || strings.Contains(r.Reason, "\n") {
				t.Errorf("reason %q want %q", r.Reason, tc.reasonHas)
			}
		})
	}
}
