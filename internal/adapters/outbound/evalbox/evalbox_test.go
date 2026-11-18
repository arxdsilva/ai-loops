package evalbox

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/evalrun"
)

// evalsDir is the repo's evals/ directory: the real scenarios.
var evalsDir = filepath.Join("..", "..", "..", "..", "evals")

func recorded(name, variant string) string {
	return filepath.Join("testdata", "runs", name, variant)
}

// TestScenarios_RecordedRuns checks every shipped scenario against a
// recorded passing run and a recorded failing run.
func TestScenarios_RecordedRuns(t *testing.T) {
	cat := Catalog{Root: evalsDir}
	names, err := cat.Names()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gate", "planted-bug", "scope", "test-first"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("scenarios = %v, want %v", names, want)
	}
	svc := evalrun.New(cat, nil, nil, Runs{})
	for _, name := range names {
		for variant, want := range map[string]bool{"pass": true, "fail": false} {
			t.Run(name+"/"+variant, func(t *testing.T) {
				out, err := svc.Recorded(name, recorded(name, variant))
				if err != nil {
					t.Fatal(err)
				}
				if out.OK != want {
					t.Fatalf("OK = %v, want %v: %+v", out.OK, want, out.Checks)
				}
			})
		}
	}
}

// fakeAgent writes a shell script that plays the agent: it copies the
// passing recording of the scenario named in its sandbox directory into
// .loops/runs/, as a real run would leave it.
func fakeAgent(t *testing.T) string {
	t.Helper()
	runs, err := filepath.Abs(filepath.Join("testdata", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "agent.sh")
	body := `#!/bin/sh
set -e
name=$(basename "$PWD" | sed -e 's/^loops-eval-//' -e 's/-[^-]*$//')
mkdir -p .loops/runs
cp -R "` + runs + `/$name/pass" ".loops/runs/20261118-0000-$name"
test -n "$1"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

// TestLive_AllScenarios_FakeAgent runs the whole live path (sandbox, agent,
// run discovery, evaluation) with a fake agent instead of a model.
func TestLive_AllScenarios_FakeAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the agent")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cat := Catalog{Root: evalsDir}
	var log bytes.Buffer
	svc := evalrun.New(cat,
		Sandbox{Fixture: filepath.Join(evalsDir, "fixture"), TempDir: t.TempDir()},
		Agent{Template: fakeAgent(t) + " {prompt}", Plugin: "/plugin", Budget: "1", Out: &log},
		Runs{})
	names, _ := cat.Names()
	for _, name := range names {
		out, err := svc.Live(context.Background(), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !out.OK || out.AgentErr != "" || out.Run == "" {
			t.Fatalf("%s: %+v (agent log: %s)", name, out, log.String())
		}
	}
}

func TestCatalog_Load(t *testing.T) {
	spec, err := Catalog{Root: evalsDir}.Load("gate")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spec.Prompt, TicketPlaceholder) || !strings.Contains(spec.Prompt, "Ship the refund rule") {
		t.Fatalf("prompt not filled: %q", spec.Prompt)
	}
	if err := spec.Scenario.Validate(); err != nil {
		t.Fatal(err)
	}
}

func writeScenario(t *testing.T, root, name, scenario string) {
	t.Helper()
	d := filepath.Join(root, "scenarios", name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	for f, body := range map[string]string{"scenario.json": scenario, "ticket.md": "T", "prompt.md": "do {{ticket}}"} {
		if err := os.WriteFile(filepath.Join(d, f), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatalog_Errors(t *testing.T) {
	root := t.TempDir()
	writeScenario(t, root, "typo", `{"name":"typo","assertions":[{"kind":"scope","min_lane":2}]}`)
	writeScenario(t, root, "renamed", `{"name":"other","assertions":[{"kind":"scope"}]}`)
	cat := Catalog{Root: root}
	for name, want := range map[string]string{
		"typo":    "unknown field",
		"renamed": `names "other"`,
		"missing": "no scenario",
		"../x":    "bad scenario name",
	} {
		if _, err := cat.Load(name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Load(%q) err = %v, want %q", name, err, want)
		}
	}
	if _, err := (Catalog{Root: filepath.Join(root, "nope")}).Names(); err == nil {
		t.Error("Names on a missing dir: want an error")
	}
}

func TestSandbox_PrepareAppliesOverlayUncommitted(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cat := Catalog{Root: evalsDir}
	spec, err := cat.Load("planted-bug")
	if err != nil {
		t.Fatal(err)
	}
	box := Sandbox{Fixture: filepath.Join(evalsDir, "fixture"), TempDir: t.TempDir()}
	dir, err := box.Prepare(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "internal", "domain", "pricing", "bulk.go")); err != nil {
		t.Fatalf("overlay not applied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(SeedDir), "W1-pricing.md")); err != nil {
		t.Fatalf("seed not copied: %v", err)
	}
	st, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(st), "bulk.go") {
		t.Fatalf("overlay should be uncommitted, status:\n%s", st)
	}
	if err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "-q", BaseTag).Run(); err != nil {
		t.Fatalf("fixture commit not tagged %s: %v", BaseTag, err)
	}
	if real, _ := filepath.EvalSymlinks(dir); real != dir {
		t.Fatalf("sandbox path %q is not resolved (%q)", dir, real)
	}
	if err := box.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("sandbox not removed: %v", err)
	}
}

func TestSandbox_PrepareFailsCleanly(t *testing.T) {
	tmp := t.TempDir()
	box := Sandbox{Fixture: filepath.Join(tmp, "no-fixture"), TempDir: tmp}
	if _, err := box.Prepare(context.Background(), evalrun.Spec{}); err == nil {
		t.Fatal("want an error for a missing fixture")
	}
	if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
		t.Fatalf("failed sandbox left behind: %v", entries)
	}
}

func TestRuns(t *testing.T) {
	project := t.TempDir()
	r := Runs{}
	if d, err := r.Latest(project); err != nil || d != "" {
		t.Fatalf("Latest with no runs = %q, %v", d, err)
	}
	for _, id := range []string{"20261118-0900-a", "20261118-1000-b"} {
		if err := os.MkdirAll(filepath.Join(project, ".loops", "runs", id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := r.Latest(project)
	if err != nil || filepath.Base(latest) != "20261118-1000-b" {
		t.Fatalf("Latest = %q, %v", latest, err)
	}
	if evs, err := r.Events(latest); err != nil || evs != nil {
		t.Fatalf("Events of an empty run = %v, %v", evs, err)
	}
	for f, body := range map[string]string{"plan.v2.md": "two", "plan.v10.md": "ten", "qa/x.md": "repro"} {
		p := filepath.Join(latest, filepath.FromSlash(f))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if plan, err := r.LatestPlan(latest); err != nil || plan != "ten" {
		t.Fatalf("LatestPlan = %q, %v; want the highest version", plan, err)
	}
	if body, ok := r.Read(latest, "qa/x.md"); !ok || body != "repro" {
		t.Fatalf("Read = %q, %v", body, ok)
	}
	if _, ok := r.Read(latest, "../../../etc/passwd"); ok {
		t.Fatal("Read must not leave the run dir")
	}
	if plan, err := r.LatestPlan(project); err != nil || plan != "" {
		t.Fatalf("LatestPlan with none = %q, %v", plan, err)
	}
	evs, err := r.Events(recorded("gate", "pass"))
	if err != nil || len(evs) == 0 {
		t.Fatalf("Events = %d, %v", len(evs), err)
	}
}

func TestAgent_Argv(t *testing.T) {
	a := Agent{Plugin: "/p", Budget: "3"}
	argv, err := a.Argv("/sb", "fix it now")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"claude", "-p", "fix it now", "--plugin-dir", "/p", "--permission-mode", "bypassPermissions", "--max-budget-usd", "3"}
	if strings.Join(argv, "|") != strings.Join(want, "|") {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	if _, err := (Agent{Template: "claude --print"}).Argv("/sb", "x"); err == nil {
		t.Fatal("a template without {prompt}: want an error")
	}
	if argv, _ := (Agent{Template: "run {dir} {prompt}"}).Argv("/sb", "x y"); len(argv) != 3 || argv[1] != "/sb" || argv[2] != "x y" {
		t.Fatalf("argv = %q", argv)
	}
}

func TestAgent_RunStripsSessionEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("CLAUDE_PROJECT_DIR", "/caller")
	t.Setenv("LOOPS_EVAL_KEEP_ME", "yes")
	dir := t.TempDir()
	script := filepath.Join(dir, "env.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"project=${CLAUDE_PROJECT_DIR:-unset} keep=$LOOPS_EVAL_KEEP_ME arg=$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := (Agent{Template: script + " {prompt}", Out: &out}).Run(context.Background(), dir, "hi", time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "project=unset keep=yes arg=hi" {
		t.Fatalf("agent saw %q", got)
	}
}

func TestAgent_RunReportsFailureAndTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	fail := filepath.Join(dir, "fail.sh")
	slow := filepath.Join(dir, "slow.sh")
	_ = os.WriteFile(fail, []byte("#!/bin/sh\nexit 3\n"), 0o755)
	_ = os.WriteFile(slow, []byte("#!/bin/sh\nexec sleep 5\n"), 0o755)
	if err := (Agent{Template: fail + " {prompt}"}).Run(context.Background(), dir, "x", time.Minute); err == nil || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("err = %v, want exit status 3", err)
	}
	if err := (Agent{Template: slow + " {prompt}"}).Run(context.Background(), dir, "x", 50*time.Millisecond); err == nil || !strings.Contains(err.Error(), "stopped after") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if err := (Agent{Template: "nope"}).Run(context.Background(), dir, "x", time.Minute); err == nil {
		t.Fatal("bad template: want an error")
	}
}
