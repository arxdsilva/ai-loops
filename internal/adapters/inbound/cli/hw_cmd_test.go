package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/hw"
)

type hwcmdProbe struct {
	f   hw.Facts
	err error
	dir string
}

func (p *hwcmdProbe) Probe(_ context.Context, dir string) (hw.Facts, error) {
	p.dir = dir
	return p.f, p.err
}

const hwcmdGiB = uint64(1) << 30

func hwcmdSetup(t *testing.T) (*rcEnv, *hwcmdProbe) {
	t.Helper()
	e := rcSetup(t, "")
	p := &hwcmdProbe{f: hw.Facts{OS: "linux", Arch: "amd64", LogicalCPUs: 16, GOMAXPROCS: 16,
		TotalRAMBytes: 16 * hwcmdGiB, AvailRAMBytes: 11*hwcmdGiB + hwcmdGiB/5, Load1: 0.5, DiskFreeBytes: 100 * hwcmdGiB, GoVersion: "go1.25"}}
	e.app.HW = p
	return e, p
}

func TestHWText(t *testing.T) {
	e, p := hwcmdSetup(t)
	if code := e.do("hw"); code != 0 {
		t.Fatal(code, e.err)
	}
	out := e.out.String()
	if p.dir != "/proj" {
		t.Errorf("dir %q", p.dir)
	}
	if n := strings.Count(out, "\n"); n > 15 || !strings.Contains(out, "RAM-bound: 11.2 GiB available → 8 agents") || !strings.Contains(out, "16 logical") {
		t.Fatalf("%d lines:\n%s", n, out)
	}
}

func TestHWJSON(t *testing.T) {
	e, _ := hwcmdSetup(t)
	e.app.Parallel = hw.Knobs{MaxAgents: 5}
	if code := e.do("hw", "--json"); code != 0 {
		t.Fatal(code, e.err)
	}
	var got struct {
		Facts          hw.Facts          `json:"facts"`
		Recommendation hw.Recommendation `json:"recommendation"`
		Knobs          hw.Knobs          `json:"knobs"`
	}
	if err := json.Unmarshal(e.out.Bytes(), &got); err != nil {
		t.Fatal(err, e.out)
	}
	if got.Facts.LogicalCPUs != 16 || got.Recommendation.MaxParallelAgents != 5 || got.Knobs.MaxAgents != 5 {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(e.out.String(), "\n  ") {
		t.Error("not indented")
	}
}

func TestHWWrite(t *testing.T) {
	e, _ := hwcmdSetup(t)
	if code := e.do("hw", "--write"); code != 1 || !strings.Contains(e.err.String(), "no active run") {
		t.Fatalf("code %d err %q", code, e.err)
	}
	e.do("run", "start", "--title", "x")
	if code := e.do("hw", "--write"); code != 0 {
		t.Fatal(code, e.err)
	}
	dir := e.log.Dir(e.ptr.id)
	js, err := os.ReadFile(filepath.Join(dir, "research", "hardware.json"))
	if err != nil || !json.Valid(js) {
		t.Fatal(err, js)
	}
	md, err := os.ReadFile(filepath.Join(dir, "research", "hardware.md"))
	if err != nil || string(md) != e.out.String() {
		t.Fatalf("%v md=%q out=%q", err, md, e.out)
	}
}

func TestHWErrorsAndFlags(t *testing.T) {
	e, p := hwcmdSetup(t)
	if e.do("hw", "--bogus") != 2 {
		t.Fatal("want usage code 2")
	}
	p.err = errors.New("boom")
	if code := e.do("hw"); code != 1 || !strings.Contains(e.err.String(), "boom") {
		t.Fatal(code, e.err)
	}
	p.err = nil
	p.f = hw.Facts{Notes: []string{"ram: nope"}}
	if code := e.do("hw"); code != 0 || !strings.Contains(e.out.String(), "unknown") || !strings.Contains(e.out.String(), "ram: nope") {
		t.Fatal(code, e.out)
	}
}
