package hook

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

type fakeSrc struct {
	v     string
	err   error
	calls int
}

func (f *fakeSrc) Latest(context.Context) (string, error) { f.calls++; return f.v, f.err }

type memStore struct{ st update.State }

func (m *memStore) Load() (update.State, error) { return m.st, nil }
func (m *memStore) Save(s update.State) error   { m.st = s; return nil }

type clk struct{}

func (clk) Now() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }

type slowUpdater struct{}

func (slowUpdater) Check(ctx context.Context, _ string, _ bool) (*updatecheck.Notice, error) {
	time.Sleep(3 * time.Second)
	return &updatecheck.Notice{Current: "0.3.0", Latest: "v9.0.0"}, nil
}

// unescape undoes JSON's \u0026 so command text can be compared verbatim.
func unescape(s string) string { return strings.ReplaceAll(s, `\u0026`, "&") }

func withUpdate(src *fakeSrc) *env {
	e := setup("")
	e.vars["LOOPS_PLUGIN_ROOT"] = "/plug"
	e.h.d.Update = updatecheck.New(src, &memStore{}, clk{})
	e.h.d.Version = "0.3.0"
	return e
}

func TestUpdateNoticeOncePerVersion(t *testing.T) {
	src := &fakeSrc{v: "v0.4.0"}
	e := withUpdate(src)
	want := "loops v0.4.0 is available (you have v0.3.0). Tell the user once: update with: " + updatecheck.Command("claude")
	_, o, er := e.runP("claude", "session-start", "{}")
	if er != "" || !strings.Contains(unescape(o), want) {
		t.Fatalf("%q / %q", o, er)
	}
	_, o, _ = e.runP("claude", "session-start", "{}")
	if strings.Contains(o, "is available") || src.calls != 1 {
		t.Fatalf("repeat notice: %q calls=%d", o, src.calls)
	}
}

func TestUpdateSystemMessage(t *testing.T) {
	for _, p := range allPlatforms {
		e := withUpdate(&fakeSrc{v: "v0.4.0"})
		_, o, _ := e.runP(p, "session-start", "{}")
		var m map[string]any
		if err := json.Unmarshal([]byte(o), &m); err != nil {
			t.Fatalf("%s: %q", p, o)
		}
		_, has := m["systemMessage"]
		wantMsg := p == "claude" || p == "codex" || p == "copilot"
		if has != wantMsg {
			t.Errorf("%s: systemMessage=%v want %v (%s)", p, has, wantMsg, o)
		}
		if wantMsg {
			if m["systemMessage"] != "loops v0.4.0 available (you have v0.3.0) — update: "+updatecheck.Command(p) {
				t.Errorf("%s: %v", p, m["systemMessage"])
			}
			if _, ok := m["hookSpecificOutput"]; !ok && p != "copilot" {
				t.Errorf("%s: no hookSpecificOutput", p)
			}
		}
		if !strings.Contains(o, "is available (you have v0.3.0)") {
			t.Errorf("%s: context missing: %s", p, o)
		}
	}
}

func TestUpdateWithoutPluginRoot(t *testing.T) {
	e := withUpdate(&fakeSrc{v: "v0.4.0"})
	delete(e.vars, "LOOPS_PLUGIN_ROOT")
	_, o, _ := e.runP("claude", "session-start", "{}")
	if !strings.Contains(o, "loops v0.4.0 is available") || strings.Contains(o, "plugin active") {
		t.Fatalf("%q", o)
	}
}

func TestUpdateOptOuts(t *testing.T) {
	for name, mut := range map[string]func(*env){
		"env":     func(e *env) { e.vars["LOOPS_NO_UPDATE_CHECK"] = "1" },
		"ci true": func(e *env) { e.vars["CI"] = "true" },
		"ci 1":    func(e *env) { e.vars["CI"] = "1" },
		"config":  func(e *env) { e.h.d.UpdateOff = true },
		"no svc":  func(e *env) { e.h.d.Update = nil },
		"no ver":  func(e *env) { e.h.d.Version = "" },
	} {
		src := &fakeSrc{v: "v0.4.0"}
		e := withUpdate(src)
		mut(e)
		_, o, _ := e.runP("claude", "session-start", "{}")
		if strings.Contains(o, "available") || src.calls != 0 {
			t.Errorf("%s: %q calls=%d", name, o, src.calls)
		}
	}
	e := withUpdate(&fakeSrc{v: "v0.4.0"})
	e.vars["CI"] = "false"
	if _, o, _ := e.runP("claude", "session-start", "{}"); !strings.Contains(o, "available") {
		t.Fatalf("CI=false should check: %q", o)
	}
}

func TestUpdateNetworkFailureSilent(t *testing.T) {
	e := withUpdate(&fakeSrc{err: errors.New("offline")})
	_, o, er := e.runP("claude", "session-start", "{}")
	if er != "" || strings.Contains(o, "available") || strings.Contains(o, "systemMessage") {
		t.Fatalf("%q / %q", o, er)
	}
}

func TestUpdateBudget(t *testing.T) {
	e := withUpdate(nil)
	e.h.d.Update = slowUpdater{}
	start := time.Now()
	_, o, _ := e.runP("claude", "session-start", "{}")
	if d := time.Since(start); d > 2500*time.Millisecond || strings.Contains(o, "v9.0.0") {
		t.Fatalf("took %v: %q", d, o)
	}
}
