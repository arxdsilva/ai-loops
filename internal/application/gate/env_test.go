package gate

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// envCmd records which commands ran with which extra environment.
type envCmd struct {
	fakeCmd
	envCalls map[string][]string
}

func (e *envCmd) RunEnv(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	if e.envCalls == nil {
		e.envCalls = map[string][]string{}
	}
	e.envCalls[name+" "+strings.Join(args, " ")] = env
	return e.fakeCmd.Run(ctx, dir, name, args...)
}

func TestRunPassesEnvToEveryCommand(t *testing.T) {
	f := &envCmd{fakeCmd: fakeCmd{has: map[string]bool{"staticcheck": true, "govulncheck": true}}}
	r := &fakeRec{}
	s := New(f, fakeArch{ok: true}, r, &tick{}).WithCoverage(&fakeCov{module: "example.com/m"}, CoverageConfig{})
	env := []string{"GOCACHE=/run/go/cache", "GOTMPDIR=/run/go/tmp"}
	if _, err := s.Run(context.Background(), opts(Options{Root: "/r", Env: env, Vuln: true})); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) == 0 || len(f.envCalls) != len(f.calls) {
		t.Fatalf("calls %v env calls %v", f.calls, f.envCalls)
	}
	for c, got := range f.envCalls {
		if !slices.Equal(got, env) {
			t.Errorf("%s ran with %v", c, got)
		}
	}
}

func TestRunWithoutEnvUsesPlainRun(t *testing.T) {
	f := &envCmd{}
	s, _ := setup(&f.fakeCmd, fakeArch{ok: true})
	s.cmd = f
	if _, err := s.Run(context.Background(), opts(Options{Root: "/r"})); err != nil {
		t.Fatal(err)
	}
	if len(f.envCalls) != 0 || len(f.calls) == 0 {
		t.Fatalf("env calls %v", f.envCalls)
	}
}

func TestRunEnvFallsBackWithoutEnvCommander(t *testing.T) {
	f := &fakeCmd{}
	s, _ := setup(f, fakeArch{ok: true})
	if _, err := s.Run(context.Background(), opts(Options{Root: "/r", Env: []string{"GOCACHE=/x"}})); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) == 0 {
		t.Fatal("no calls")
	}
}
