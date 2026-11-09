package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

type fakeHW struct{ f hw.Facts }

func (h fakeHW) Probe(context.Context, string) (hw.Facts, error) { return h.f, nil }

const planHdr = "### Wave 1\n| id | role | model | job | owns | x | y | z |\n|---|---|---|---|---|---|---|---|\n"

func planRow(id, owns string) string {
	return "| " + id + " | eng | s | j | " + owns + " | - | t | C1 |\n"
}

func writePlan(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "PLAN.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanCheck(t *testing.T) {
	e := rcSetup(t, "")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok := writePlan(t, dir, planHdr+planRow("W1-a", "a/**")+planRow("W1-b", "b/**"))
	if c := e.do("plan", "check", ok); c != 0 || !strings.Contains(e.out.String(), "cap 100 (default)") || !strings.Contains(e.out.String(), "peak 2") {
		t.Fatal(c, e.out, e.err)
	}
	if c := e.do("plan", "check", ok, "--cap", "1"); c != 1 || !strings.Contains(e.out.String(), "cap") {
		t.Fatal(c, e.out)
	}
	bad := writePlan(t, filepath.Join(dir, "sub"), planHdr+planRow("W1-a", "a/**")+planRow("W1-b", "a/x.go"))
	if c := e.do("plan", "check", bad, "--json"); c != 1 || !strings.Contains(e.out.String(), `"rule": "overlap"`) || !strings.Contains(e.out.String(), `"cap_source"`) {
		t.Fatal(c, e.out)
	}
	if c := e.do("plan", "check", ok, "--json"); c != 0 || !strings.Contains(e.out.String(), `"findings": []`) {
		t.Fatal(c, e.out)
	}
}

func TestPlanUsage(t *testing.T) {
	e := rcSetup(t, "")
	for _, args := range [][]string{{"plan"}, {"plan", "check"}, {"plan", "check", "/nonexistent/x.md"}, {"plan", "check", "--bogus"}} {
		if c := e.do(args...); c != 2 {
			t.Errorf("%v: %d", args, c)
		}
	}
}

func TestPlanRelativeToRunDirAndHardwareFile(t *testing.T) {
	e := rcSetup(t, "")
	if e.do("run", "start", "--title", "x") != 0 {
		t.Fatal(e.err)
	}
	dir := e.log.Dir(e.ptr.id)
	if err := os.MkdirAll(filepath.Join(dir, "research"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := `{"recommendation":{"max_parallel_agents":1,"reason":"RAM-bound: tiny"}}`
	if err := os.WriteFile(filepath.Join(dir, "research", "hardware.json"), []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	writePlan(t, dir, planHdr+planRow("W1-a", "a/**")+planRow("W1-b", "b/**"))
	c := e.do("plan", "check", "PLAN.md")
	if c != 1 || !strings.Contains(e.out.String(), "cap 1 (hardware: RAM-bound: tiny)") {
		t.Fatal(c, e.out, e.err)
	}
}

func TestEffectiveCap(t *testing.T) {
	e := rcSetup(t, "")
	a := e.app
	ctx := context.Background()
	a.Parallel.MaxAgents = 50
	if n, s := a.effectiveCap(ctx, ""); n != 50 || !strings.Contains(s, "parallel") {
		t.Fatal(n, s)
	}
	a.Limits.MaxAgents = 7
	if n, s := a.effectiveCap(ctx, ""); n != 7 || !strings.Contains(s, "limits") {
		t.Fatal(n, s)
	}
	a.HW = fakeHW{hw.Facts{LogicalCPUs: 2, GOMAXPROCS: 2, TotalRAMBytes: 8 << 30, AvailRAMBytes: 4 << 30}}
	if n, s := a.effectiveCap(ctx, ""); n > 7 || n < 1 || s == "" {
		t.Fatal(n, s)
	}
}
