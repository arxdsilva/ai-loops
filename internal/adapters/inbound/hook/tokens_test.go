package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

func TestSumTranscriptFixtures(t *testing.T) {
	for name, want := range map[string]int64{
		// msg_1 is split over two lines carrying the same usage (counted once, latest):
		// 10+1000+0+50 = 1060; msg_2: 20+0+1000+100 = 1120.
		"claude.jsonl": 2180,
		// 3+4, prompt+completion 15, total 30, 1+1 next to a string message.
		"toplevel.jsonl": 54,
		// cumulative total_token_usage: the last (highest) total.
		"codex.jsonl": 3500,
		"empty.jsonl": 0,
	} {
		f, err := os.Open(filepath.Join("testdata", "transcripts", name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := sumTranscript(f)
		f.Close()
		if err != nil || got != want {
			t.Errorf("%s: got %d, %v want %d", name, got, err, want)
		}
	}
}

func TestSumTranscriptLongLine(t *testing.T) {
	long := `{"type":"user","message":{"content":"` + strings.Repeat("x", 300<<10) + `"}}` + "\n"
	got, err := sumTranscript(strings.NewReader(long + `{"usage":{"input_tokens":5,"output_tokens":6}}`)) // no trailing newline
	if err != nil || got != 11 {
		t.Fatalf("got %d, %v", got, err)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, os.ErrPermission }

func TestTranscriptTokensFailsOpen(t *testing.T) {
	if _, err := sumTranscript(errReader{}); err == nil {
		t.Fatal("read error must surface from sumTranscript")
	}
	dir := t.TempDir()
	for _, p := range []string{"", filepath.Join(dir, "missing.jsonl"), dir} {
		if n := transcriptTokens(p, maxTranscriptBytes); n != 0 {
			t.Errorf("%q: %d", p, n)
		}
	}
	p := filepath.Join(dir, "t.jsonl")
	body := `{"usage":{"input_tokens":1,"output_tokens":1}}` + "\n" + `{"usage":{"input_tokens":100,"output_tokens":100}}` + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := transcriptTokens(p, maxTranscriptBytes); n != 202 {
		t.Fatalf("full read: %d", n)
	}
	if n := transcriptTokens(p, int64(len(body)/2)); n != 2 { // cap mid-file: only complete lines count
		t.Fatalf("capped read: %d", n)
	}
}

func TestSubagentStopRecordsTokens(t *testing.T) {
	src, err := filepath.Abs(filepath.Join("testdata", "transcripts", "claude.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	e := setup("r")
	e.run("trace-agent", `{"hook_event_name":"SubagentStop","agent_id":"a1","agent_type":"loops:planner","transcript_path":"/main/session.jsonl","agent_transcript_path":`+quote(src)+`}`)
	e.run("trace-agent", `{"hook_event_name":"subagentStop","subagentId":"","subagent_id":"a2","subagent_type":"x","agentTranscriptPath":`+quote(src)+`}`)
	e.run("trace-agent", `{"hook_event_name":"SubagentStop","agent_id":"a3","agent_transcript_path":"/nope/missing.jsonl"}`)
	e.run("trace-agent", `{"hook_event_name":"SubagentStop","agent_id":"a4","transcript_path":`+quote(src)+`}`) // the lead's transcript is never counted
	e.run("trace-agent", `{"hook_event_name":"SubagentStart","agent_id":"a5","agent_transcript_path":`+quote(src)+`}`)
	want := []int64{2180, 2180, 0, 0, 0}
	if len(e.rec.evs) != len(want) {
		t.Fatalf("%+v", e.rec.evs)
	}
	for i, w := range want {
		if ev := e.rec.evs[i]; ev.Tokens != w {
			t.Errorf("event %d (%s %s): tokens %d want %d", i, ev.Kind, ev.Agent, ev.Tokens, w)
		}
	}
	if s := e.rec.evs[0]; s.Kind != run.KindAgentStop || s.Agent != "a1" || s.Role != "planner" || !*s.OK {
		t.Fatalf("stop event changed: %+v", s)
	}
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }
