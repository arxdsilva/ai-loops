package gate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

type fakeCmd struct {
	calls []string
	fail  map[string]string // "go test" -> output (fails)
	out   map[string]string // success output
	has   map[string]bool
}

func (f *fakeCmd) Run(_ context.Context, dir, name string, args ...string) (string, error) {
	key := name
	if len(args) > 0 && name == "go" {
		key = name + " " + args[0]
	}
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if o, bad := f.fail[key]; bad {
		return o, errors.New("exit 1")
	}
	return f.out[key], nil
}
func (f *fakeCmd) Has(n string) bool {
	if v, ok := f.has[n]; ok {
		return v
	}
	return n == "go" || n == "gofmt" // the toolchain is always there
}

type fakeArch struct {
	text string
	ok   bool
}

func (a fakeArch) Check(context.Context, string) (string, bool, error) { return a.text, a.ok, nil }

type fakeRec struct {
	evs []run.Event
	err error
}

func (r *fakeRec) Record(_ context.Context, e run.Event) error {
	r.evs = append(r.evs, e)
	return r.err
}

type tick struct{ t time.Time }

func (c *tick) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

func setup(f *fakeCmd, a fakeArch) (*service, *fakeRec) {
	r := &fakeRec{}
	return New(f, a, r, &tick{}).WithCoverage(&fakeCov{module: "example.com/m"}, CoverageConfig{}), r
}

// goPackOnce is the reference Go pack (specs/languages/go/pack.json).
var goPackOnce = sync.OnceValues(func() (pack.Pack, error) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "specs", "languages", "go", "pack.json"))
	if err != nil {
		return pack.Pack{}, err
	}
	return pack.Parse(b)
})

func goPack() pack.Pack {
	p, err := goPackOnce()
	if err != nil {
		panic(err)
	}
	return p
}

// goOpts adds the Go pack at the project root, as the CLI detects it.
func goOpts(o Options) Options {
	if o.Packs == nil {
		o.Packs = []langpack.Detected{{Pack: goPack(), ID: "go", Name: "Go", Root: "."}}
	}
	return o
}

// opts are full-gate options with a coverprofile path, as the CLI passes them.
func opts(o Options) Options {
	if o.CoverProfile == "" {
		o.CoverProfile = "/tmp/cover.out"
	}
	return goOpts(o)
}

func names(r Result) string {
	var s []string
	for _, c := range r.Checks {
		st := "ok"
		if c.Skipped {
			st = "skip"
		} else if !c.OK {
			st = "FAIL"
		}
		s = append(s, c.Name+":"+st)
	}
	return strings.Join(s, " ")
}

func TestFullGreen(t *testing.T) {
	f := &fakeCmd{out: map[string]string{"go test": "ok  \tpkg\t0.1s\n"}, has: map[string]bool{"staticcheck": true}}
	s, rec := setup(f, fakeArch{ok: true})
	res, err := s.Run(context.Background(), opts(Options{Root: "/p"}))
	if err != nil || !res.OK {
		t.Fatal(res, err)
	}
	if got := names(res); got != "build:ok gofmt:ok vet:ok staticcheck:ok arch:ok test:ok coverage:ok govulncheck:skip gosec:skip" {
		t.Fatal(got)
	}
	want := []string{"go build ./...", "gofmt -l .", "go vet ./...", "staticcheck ./...", "go test -race -count=1 -covermode=atomic -coverprofile=/tmp/cover.out ./..."}
	for i, w := range want {
		if f.calls[i] != w {
			t.Errorf("call %d: %q want %q", i, f.calls[i], w)
		}
	}
	if len(rec.evs) != 8 { // 7 checks + all
		t.Fatalf("events %d", len(rec.evs))
	}
	if rec.evs[6].Name != "coverage" || rec.evs[6].Note != "no new statements" || !*rec.evs[6].OK {
		t.Fatalf("%+v", rec.evs[6])
	}
	last := rec.evs[7]
	if last.Name != run.GateAll || !*last.OK || last.Kind != run.KindGate {
		t.Fatalf("%+v", last)
	}
	if rec.evs[5].Name != "test" || rec.evs[5].Note != "ok  \tpkg\t0.1s" || rec.evs[5].DurationMS != 1000 {
		t.Fatalf("%+v", rec.evs[5])
	}
}

func TestFailuresAndFailFast(t *testing.T) {
	f := &fakeCmd{fail: map[string]string{"go vet": "vet: bad\n"}, out: map[string]string{"gofmt": "a.go\nvendor/x.go\ntestdata/y.go\n"}}
	s, rec := setup(f, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), opts(Options{Root: "/p"}))
	if res.OK || names(res) != "build:ok gofmt:FAIL vet:FAIL staticcheck:skip arch:ok test:ok coverage:ok govulncheck:skip gosec:skip" {
		t.Fatal(names(res))
	}
	if !strings.Contains(res.Checks[1].Output, "a.go") || strings.Contains(res.Checks[1].Output, "vendor") {
		t.Fatalf("%q", res.Checks[1].Output)
	}
	if all := rec.evs[len(rec.evs)-1]; all.Name != run.GateAll || *all.OK {
		t.Fatalf("%+v", all)
	}

	f2 := &fakeCmd{fail: map[string]string{"go build": "boom"}}
	s2, rec2 := setup(f2, fakeArch{ok: true})
	res2, _ := s2.Run(context.Background(), opts(Options{FailFast: true}))
	if res2.OK || len(f2.calls) != 1 || len(rec2.evs) != 2 { // build + all
		t.Fatalf("%v %d", f2.calls, len(rec2.evs))
	}
}

func TestScopedNeverRecordsAll(t *testing.T) {
	f := &fakeCmd{}
	s, rec := setup(f, fakeArch{ok: true})
	res, _ := s.Run(context.Background(), goOpts(Options{Pkgs: []string{"./internal/domain/..."}, Only: []string{"build", "gofmt", "vet"}}))
	if !res.OK || names(res) != "build:ok gofmt:ok vet:ok staticcheck:skip arch:skip test:skip coverage:skip govulncheck:skip gosec:skip" {
		t.Fatal(names(res))
	}
	if f.calls[1] != "gofmt -l internal/domain" || f.calls[0] != "go build ./internal/domain/..." {
		t.Fatal(f.calls)
	}
	for _, e := range rec.evs {
		if e.Name == run.GateAll {
			t.Fatal("all recorded")
		}
	}
	// Pkgs only (no Only) also never records all.
	s2, rec2 := setup(&fakeCmd{}, fakeArch{ok: true})
	_, _ = s2.Run(context.Background(), goOpts(Options{Pkgs: []string{"./x"}}))
	for _, e := range rec2.evs {
		if e.Name == run.GateAll {
			t.Fatal("all recorded")
		}
	}
}

func TestSkipArchVulnAndRecorderError(t *testing.T) {
	f := &fakeCmd{has: map[string]bool{"govulncheck": true}}
	r := &fakeRec{err: errors.New("no active loops run")}
	s := New(f, fakeArch{text: "arch bad", ok: false}, r, &tick{}).WithCoverage(&fakeCov{}, CoverageConfig{})
	res, err := s.Run(context.Background(), opts(Options{Skip: []string{"test", "coverage"}, Vuln: true}))
	if err != nil {
		t.Fatal(err)
	}
	// coverage is required: --skip cannot drop it, and without the test
	// check's profile it fails.
	if got := names(res); got != "build:ok gofmt:ok vet:ok staticcheck:skip arch:FAIL test:skip coverage:FAIL govulncheck:ok gosec:skip" {
		t.Fatal(got)
	}
	if res.Checks[4].Output != "arch bad" {
		t.Fatal(res.Checks[4].Output)
	}
}

func TestTrimTestOutput(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "noise %d\n", i)
	}
	b.WriteString("--- FAIL: TestX\n    x_test.go:5: bad\nFAIL\n")
	sig := signalFunc(`^FAIL|--- FAIL|panic|DATA RACE|_test\.go:`)
	got := trim(b.String(), sig)
	lines := strings.Split(got, "\n")
	if len(lines) != 60 || lines[0] != "--- FAIL: TestX" {
		t.Fatalf("%d %q", len(lines), lines[0])
	}
	if n := len(strings.Split(trim(b.String(), nil), "\n")); n != 60 {
		t.Fatal(n)
	}
	long := strings.Repeat("x", 300)
	if len(firstLine("\n  "+long)) != 200 {
		t.Fatal("note len")
	}
}

func TestCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, _ := setup(&fakeCmd{}, fakeArch{ok: true})
	if _, err := s.Run(ctx, goOpts(Options{})); err == nil {
		t.Fatal("want ctx error")
	}
}

func TestFmtDirs(t *testing.T) {
	got := fmtDirs([]string{"./a/...", "b", "./..."})
	if strings.Join(got, ",") != "a,b,." {
		t.Fatal(got)
	}
}

func TestAllRules(t *testing.T) {
	ctx := context.Background()
	allEv := func(rec *fakeRec) *run.Event {
		for i := range rec.evs {
			if rec.evs[i].Name == run.GateAll {
				return &rec.evs[i]
			}
		}
		return nil
	}
	// CLI --skip never records all.
	s, rec := setup(&fakeCmd{}, fakeArch{ok: true})
	_, _ = s.Run(ctx, goOpts(Options{Skip: []string{"test"}}))
	if allEv(rec) != nil {
		t.Fatal("all recorded with --skip")
	}
	// Config skip of a required check records a failed all.
	for _, req := range []string{"build", "vet", "test", "coverage"} {
		s, rec = setup(&fakeCmd{}, fakeArch{ok: true})
		res, _ := s.Run(ctx, opts(Options{ConfigSkip: []string{req}}))
		ev := allEv(rec)
		if ev == nil || *ev.OK || ev.Note != "required check skipped: "+req {
			t.Fatalf("%s: %+v", req, ev)
		}
		if res.OK {
			t.Fatalf("%s: Result.OK must be false so `loops gate` exits 1 and agrees with the Stop gate", req)
		}
	}
	// Config skip of optional checks still allows a green all, stamped with the start time.
	s, rec = setup(&fakeCmd{}, fakeArch{ok: true})
	_, _ = s.Run(ctx, opts(Options{ConfigSkip: []string{"gofmt", "arch"}}))
	ev := allEv(rec)
	if ev == nil || !*ev.OK || !ev.Time.Equal(time.Time{}.Add(time.Second)) {
		t.Fatalf("all must carry the pipeline start time (first clock tick): %+v", ev)
	}
}

func TestStaleStaticcheckIsSkipped(t *testing.T) {
	for _, msg := range []string{"x: unsupported version: 5", "internal error in importing \"fmt\""} {
		f := &fakeCmd{has: map[string]bool{"staticcheck": true}, fail: map[string]string{"staticcheck": msg}}
		s, rec := setup(f, fakeArch{ok: true})
		res, _ := s.Run(context.Background(), opts(Options{}))
		c := res.Checks[3]
		if !res.OK || !c.Skipped || c.Output != StaleStaticcheckNote {
			t.Fatalf("%+v", c)
		}
		for _, e := range rec.evs {
			if e.Name == "staticcheck" {
				t.Fatal("recorded")
			}
		}
	}
	f := &fakeCmd{has: map[string]bool{"staticcheck": true}, fail: map[string]string{"staticcheck": "a.go:1: SA1000"}}
	s, _ := setup(f, fakeArch{ok: true})
	if res, _ := s.Run(context.Background(), goOpts(Options{})); res.OK {
		t.Fatal("real staticcheck failure must fail")
	}
}
