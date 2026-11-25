package hook

import (
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/workspace"
)

// raceOutcome is what the pair of PreToolUse hooks does with a command.
type raceOutcome int

const (
	raceRewritten raceOutcome = iota // toolchain-env rewrites (adding -race), test-guard allows
	raceDenied                       // test-guard denies, toolchain-env stays silent
)

// TestRaceRewriteMatrix: for every platform and command shape exactly one of
// the two PreToolUse hooks acts. Where toolchain-env rewrites a Go command it
// also adds the missing -race and test-guard stays silent; elsewhere
// test-guard denies and toolchain-env stays silent. Never both rewrite and
// deny, never neither.
func TestRaceRewriteMatrix(t *testing.T) {
	type outcome = raceOutcome
	const (
		rewritten_ = raceRewritten
		denied     = raceDenied
	)
	allRewrite := map[string]outcome{"claude": rewritten_, "gemini": rewritten_, "windsurf": rewritten_, "antigravity": rewritten_}
	cases := []struct {
		name string
		cmd  func(cache string) string
		want map[string]outcome
		// fixed is the command toolchain-env must produce before scoping.
		fixed string
	}{
		{"lone gate command, unscoped, no -race", func(string) string { return "go test ./..." },
			merge(allRewrite, map[string]outcome{"codex": rewritten_, "cursor": rewritten_, "copilot": denied, "opencode": denied}),
			"go test -race ./..."},
		{"compound unscoped, no -race", func(string) string { return "cd sub && go test -count=1 ./..." },
			merge(allRewrite, map[string]outcome{"codex": denied, "cursor": denied, "copilot": denied, "opencode": denied}),
			"cd sub && go test -race -count=1 ./..."},
		{"pipeline unscoped, no -race", func(string) string { return "go test ./... 2>&1 | tail -20" },
			merge(allRewrite, map[string]outcome{"codex": denied, "cursor": denied, "copilot": denied, "opencode": denied}),
			"go test -race ./... 2>&1 | tail -20"},
		{"already scoped, no -race", func(cache string) string { return "GOCACHE=" + cache + " go test ./..." },
			map[string]outcome{"claude": denied, "gemini": denied, "windsurf": denied, "antigravity": denied, "codex": denied, "cursor": denied, "copilot": denied, "opencode": denied},
			""},
		{"unscoped, race present", func(string) string { return "go test -race ./..." },
			merge(allRewrite, map[string]outcome{"codex": rewritten_, "cursor": rewritten_}), // scoping only, no race added
			"go test -race ./..."},
		{"explicitly race=false", func(string) string { return "go test -race=false ./..." },
			map[string]outcome{"claude": denied, "gemini": denied, "windsurf": denied, "antigravity": denied, "codex": denied, "cursor": denied, "copilot": denied, "opencode": denied},
			""},
	}
	for _, c := range cases {
		for plat, want := range c.want {
			t.Run(c.name+"/"+plat, func(t *testing.T) {
				e, rd := goEnvSetup(t, "r1")
				cache := workspace.GoCache(rd)
				cmd := c.cmd(cache)
				payload := bashPayload(plat, cmd)
				if plat == "opencode" {
					payload = bashPayload("claude", cmd)
				}
				gCode, gOut, gErr := e.runP(plat, "test-guard", payload)
				guardDenied := gCode == 2 || strings.Contains(gOut, `"decision":"deny"`)
				if gOut != "" && !guardDenied {
					t.Fatalf("test-guard must never rewrite: %q", gOut)
				}
				eCode, eOut, _ := e.runP(plat, "go-env", payload)
				envRewrote := eCode == 0 && strings.Contains(eOut, "GOCACHE=")
				switch want {
				case denied:
					if !guardDenied || strings.Contains(eOut, "updatedInput") || envRewrote {
						t.Fatalf("want deny only: guard=%v env=%q", guardDenied, eOut)
					}
				case rewritten_:
					if guardDenied {
						t.Fatalf("test-guard denied a command toolchain-env rewrites: %q %q", gOut, gErr)
					}
					if !envRewrote {
						t.Fatalf("toolchain-env did not rewrite: %d %q", eCode, eOut)
					}
					got, _, _ := rewritten(t, plat, eOut)
					if exp := guard.ScopeGo(c.fixed, cache, workspace.GoTmp(rd)); got != exp {
						t.Fatalf("rewrote to %q, want %q", got, exp)
					}
					if r := guard.CheckRace(got); r != "" {
						t.Fatalf("rewritten command still breaks the race policy: %q", got)
					}
				}
			})
		}
	}
}

func merge(a, b map[string]raceOutcome) map[string]raceOutcome {
	out := map[string]raceOutcome{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// No active run: nothing rewrites, so the guard denies.
func TestRaceRewriteNeedsActiveRun(t *testing.T) {
	e, _ := goEnvSetup(t, "")
	if c, _, er := e.runP("claude", "test-guard", bashPayload("claude", "go test ./...")); c != 2 || !strings.Contains(er, "race_guard:") {
		t.Fatalf("%d %q", c, er)
	}
}

// A non-Go pack's violation on the same line keeps the whole command denied.
func TestRaceRewriteKeepsOtherPackViolationsDenied(t *testing.T) {
	e, _, _ := langSetup("r", goDet, swiftDet)
	if c, _, er := e.run("test-guard", `{"tool_name":"Bash","tool_input":{"command":"go test ./... && swift test"}}`); c != 2 || er == "" {
		t.Fatalf("%d %q", c, er)
	}
	// toolchain-env then stays silent too: nothing rewrites a denied command.
	if c, o, er := e.run("toolchain-env", `{"tool_name":"Bash","tool_input":{"command":"go test ./... && swift test"}}`); c != 0 || o != "" || er != "" {
		t.Fatalf("%d %q %q", c, o, er)
	}
}
