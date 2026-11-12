package updatecheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/arxdsilva/ai-loops/internal/domain/update"
)

type fakeRunner struct {
	argvs  [][]string
	outs   map[int]string
	errs   map[int]error
	failAt int // step index that fails with errs; -1 none
}

func (f *fakeRunner) Run(_ context.Context, argv []string) (string, error) {
	i := len(f.argvs)
	f.argvs = append(f.argvs, argv)
	return f.outs[i], f.errs[i]
}

func newRunner() *fakeRunner {
	return &fakeRunner{outs: map[int]string{}, errs: map[int]error{}}
}

type mutClock struct{ t time.Time }

func (c *mutClock) Now() time.Time { return c.t }

func applySetup(st update.State, v string, err error, r *fakeRunner) (*Service, *fakeSrc, *fakeStore) {
	src, store := &fakeSrc{v: v, err: err}, &fakeStore{st: st}
	s := New(src, store, fakeClock{now})
	if r != nil {
		s.WithRunner(r)
	}
	return s, src, store
}

func TestApplyIgnoresThrottle(t *testing.T) { // C1
	st := update.State{LastCheck: now.Add(-time.Minute), Latest: "v0.9.0", LastNotified: "v0.9.1"}
	s, src, store := applySetup(st, "v0.9.1", nil, newRunner())
	out := s.Apply(context.Background(), "0.9.0", "claude")
	if src.calls != 1 {
		t.Fatalf("source calls = %d", src.calls)
	}
	if store.st.Latest != "v0.9.1" || !store.st.LastCheck.Equal(now) || store.st.LastNotified != "v0.9.1" {
		t.Errorf("state = %+v", store.st)
	}
	if out.Latest != "v0.9.1" || out.Current != "0.9.0" {
		t.Errorf("out = %+v", out)
	}
}

func TestApplyCheckFailed(t *testing.T) { // C2
	for _, c := range []struct {
		v   string
		err error
	}{{"", errors.New("boom\nline2")}, {"", nil}} {
		r := newRunner()
		s, _, store := applySetup(update.State{}, c.v, c.err, r)
		out := s.Apply(context.Background(), "0.9.0", "claude")
		if out.Kind != KindCheckFailed || len(r.argvs) != 0 || store.saves != 0 {
			t.Errorf("out=%+v runs=%d saves=%d", out, len(r.argvs), store.saves)
		}
		if strings.ContainsAny(out.Detail, "\n\t") || out.Detail == "" {
			t.Errorf("detail = %q", out.Detail)
		}
	}
}

func TestApplyUpToDate(t *testing.T) { // C3
	for _, v := range []string{"v0.9.0", "v0.8.0", "garbage"} {
		r := newRunner()
		s, _, store := applySetup(update.State{}, v, nil, r)
		out := s.Apply(context.Background(), "0.9.0", "claude")
		if out.Kind != KindUpToDate || len(r.argvs) != 0 || out.Manual != "" {
			t.Errorf("%s: out=%+v", v, out)
		}
		if store.saves != 1 {
			t.Errorf("%s: saves = %d", v, store.saves)
		}
	}
}

func TestApplyClaudeSuccess(t *testing.T) { // C4
	r := newRunner()
	s, _, store := applySetup(update.State{}, "v0.10.0", nil, r)
	out := s.Apply(context.Background(), "0.9.0", " Claude ")
	if out.Kind != KindUpdated || out.Platform != "claude" || out.Reload != "/reload-plugins or a new session" {
		t.Fatalf("out = %+v", out)
	}
	want := "claude plugin marketplace update ai-loops|claude plugin update loops@ai-loops"
	var got []string
	for _, a := range r.argvs {
		got = append(got, strings.Join(a, " "))
	}
	if strings.Join(got, "|") != want {
		t.Errorf("argv = %v", got)
	}
	if store.st.Applied != "v0.10.0" || !store.st.AppliedAt.Equal(now) || store.st.ApplyErr != "" {
		t.Errorf("state = %+v", store.st)
	}
	if out.Manual != Command("claude") {
		t.Errorf("manual = %q", out.Manual)
	}
}

func TestApplyFailureThenBackoff(t *testing.T) { // C5
	r := newRunner()
	r.errs[0] = errors.New("exit status 1")
	r.outs[0] = "warming up\nmarketplace unreachable\n\n"
	src, store := &fakeSrc{v: "v0.10.0"}, &fakeStore{}
	clk := &mutClock{now}
	s := New(src, store, clk).WithRunner(r)
	out := s.Apply(context.Background(), "0.9.0", "claude")
	if out.Kind != KindFailed || len(r.argvs) != 1 {
		t.Fatalf("out=%+v runs=%d", out, len(r.argvs))
	}
	if !strings.Contains(out.Detail, "claude plugin marketplace update ai-loops") || !strings.HasSuffix(out.Detail, "marketplace unreachable") {
		t.Errorf("detail = %q", out.Detail)
	}
	if store.st.ApplyErr != out.Detail {
		t.Errorf("saved err = %q", store.st.ApplyErr)
	}
	clk.t = now.Add(time.Hour)
	out = s.Apply(context.Background(), "0.9.0", "claude")
	if out.Kind != KindBackoff || !out.RetryAt.Equal(now.Add(24*time.Hour)) || !out.At.Equal(now) || len(r.argvs) != 1 {
		t.Fatalf("backoff out=%+v runs=%d", out, len(r.argvs))
	}
	if out.Detail == "" {
		t.Error("backoff detail empty")
	}
	clk.t = now.Add(24 * time.Hour)
	if out = s.Apply(context.Background(), "0.9.0", "claude"); len(r.argvs) < 3 || out.Kind == KindBackoff {
		t.Fatalf("retry out=%+v runs=%d", out, len(r.argvs))
	}
}

func TestApplyPendingAfterSuccess(t *testing.T) {
	r := newRunner()
	clk := &mutClock{now}
	s := New(&fakeSrc{v: "v0.10.0"}, &fakeStore{}, clk).WithRunner(r)
	s.Apply(context.Background(), "0.9.0", "copilot")
	clk.t = now.Add(time.Hour)
	out := s.Apply(context.Background(), "0.9.0", "copilot")
	if out.Kind != KindPending || !out.At.Equal(now) || len(r.argvs) != 2 || out.Reload != "/restart or a new session" {
		t.Fatalf("out=%+v runs=%d", out, len(r.argvs))
	}
}

func TestApplyErrNoCommand(t *testing.T) { // C6
	r := newRunner()
	r.errs[0] = fmt.Errorf("%w: claude", ErrNoCommand)
	s, _, _ := applySetup(update.State{}, "v0.10.0", nil, r)
	out := s.Apply(context.Background(), "0.9.0", "claude")
	if out.Kind != KindFailed || !strings.Contains(out.Detail, "claude not on PATH") {
		t.Errorf("out = %+v", out)
	}
}

func TestApplyManual(t *testing.T) { // C7
	for _, p := range []string{"cursor", "gemini", "codex", "opencode", ""} {
		r := newRunner()
		s, _, _ := applySetup(update.State{}, "v0.10.0", nil, r)
		out := s.Apply(context.Background(), "0.9.0", p)
		if out.Kind != KindManual || out.Manual != Command(p) || len(r.argvs) != 0 {
			t.Errorf("%q: out=%+v", p, out)
		}
	}
	s, _, _ := applySetup(update.State{}, "v0.10.0", nil, nil)
	if out := s.Apply(context.Background(), "0.9.0", "claude"); out.Kind != KindManual {
		t.Errorf("nil runner: %+v", out)
	}
}

func TestApplyTimedOut(t *testing.T) { // C9
	r := newRunner()
	r.errs[0] = fmt.Errorf("run: %w", context.DeadlineExceeded)
	s, _, store := applySetup(update.State{}, "v0.10.0", nil, r)
	out := s.Apply(context.Background(), "0.9.0", "claude")
	if out.Kind != KindFailed || !strings.Contains(out.Detail, "timed out") || store.st.Applied != "v0.10.0" {
		t.Errorf("out=%+v st=%+v", out, store.st)
	}
	r2 := newRunner()
	r2.errs[0] = errors.New("killed")
	s2, _, _ := applySetup(update.State{}, "v0.10.0", nil, r2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out := s2.Apply(ctx, "0.9.0", "claude"); out.Kind != KindFailed || !strings.Contains(out.Detail, "timed out") {
		t.Errorf("cancelled: %+v", out)
	}
}

func TestApplyStoreErrors(t *testing.T) { // C10
	s, _, store := applySetup(update.State{}, "v0.10.0", nil, newRunner())
	store.loadErr, store.saveErr = errors.New("load"), errors.New("save")
	if out := s.Apply(context.Background(), "0.9.0", "claude"); out.Kind != KindUpdated {
		t.Errorf("out = %+v", out)
	}
	if store.saves != 1 {
		t.Errorf("saves = %d", store.saves)
	}
}

func TestOneLine(t *testing.T) {
	if got := oneLine(" a\n\tb  c "); got != "a b c" {
		t.Errorf("got %q", got)
	}
	long := strings.Repeat("é", 150) // 300 bytes
	got := oneLine(long)
	if len(got) > maxDetail || !utf8.ValidString(got) || got == "" {
		t.Errorf("len=%d valid=%v", len(got), utf8.ValidString(got))
	}
	if lastLine("") != "" {
		t.Error("lastLine of empty")
	}
}
