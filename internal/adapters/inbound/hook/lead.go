package hook

// What the hooks do for the lead (the main session) so it spends fewer turns
// on bookkeeping: label the lanes it spawns, count its own tokens, and warn
// when a run starts in an already heavy session. Claude Code only: the
// payload and transcript shapes are Claude's. Everything fails open.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

const (
	// maxLeadTranscriptBytes caps how much of the lead's transcript the Stop
	// hook reads (the tail: a run is recent).
	maxLeadTranscriptBytes = 256 << 20
	// contextTailBytes is how much of the transcript's tail the run-start hint
	// reads to find the last assistant message.
	contextTailBytes = 8 << 20
	// contextHintTokens is the context size above which a run start warns.
	contextHintTokens = 150_000
)

var (
	// agentIDText finds the agent id in a text-form spawn response.
	agentIDText = regexp.MustCompile(`agentId:\s*([A-Za-z0-9_-]+)`)
	// usageKey prefilters transcript lines: only those can carry token usage.
	usageKey = []byte(`"usage"`)
	// runStartCmd matches a shell command that runs `loops run start`: the
	// bare name, an absolute path, a quoted path or a variable holding it ($L).
	runStartCmd = regexp.MustCompile(`(?:^|[\s;&|(])(?:\S*loops|\$\{?\w+\}?)["']?\s+run\s+start(?:\s|$)`)
)

// ---- auto-label (PostToolUse Agent|Task) ----

// headerValue returns the value of the `Key: value` line in a spawn prompt's
// header (skills/work/SKILL.md, "Prompt header").
func headerValue(prompt, key string) string {
	re := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(key) + `:[ \t]*(\S[^\r\n]*?)[ \t]*\r?$`)
	if m := re.FindStringSubmatch(prompt); m != nil {
		return m[1]
	}
	return ""
}

// spawnResult is the part of an Agent tool_response the label needs. A
// background spawn answers {"isAsync":true,"status":"async_launched",
// "agentId":"…","resolvedModel":"claude-haiku-5-5",…}.
type spawnResult struct {
	AgentID       string `json:"agentId"`
	AgentIDSnake  string `json:"agent_id"`
	ResolvedModel string `json:"resolvedModel"`
}

// parseSpawn returns the new agent's id and resolved model from a tool
// response: the structured object, else any text containing `agentId: <id>`.
func parseSpawn(raw json.RawMessage) (id, model string) {
	var r spawnResult
	if json.Unmarshal(raw, &r) == nil {
		id, model = firstNonEmpty(r.AgentID, r.AgentIDSnake), r.ResolvedModel
	}
	if id == "" {
		if m := agentIDText.FindStringSubmatch(string(raw)); m != nil {
			id = m[1]
		}
	}
	return id, model
}

// autoLabel records the `loops run label` event for an agent the lead just
// spawned, from the prompt header and the tool response, so the lead need not
// call `loops run label` itself. It stays silent without an `Agent id:` line
// or an agent id.
func (h *handler) autoLabel(ctx context.Context, p payload, in toolInput) {
	name := headerValue(in.Prompt, "Agent id")
	if name == "" {
		return
	}
	id, resolved := parseSpawn(p.ToolResponse)
	if id == "" {
		return
	}
	role := firstNonEmpty(headerValue(in.Prompt, "Role"), roleName(in.SubagentType))
	wave, _ := strconv.Atoi(headerValue(in.Prompt, "Wave"))
	model := firstNonEmpty(guard.ModelTier(resolved), resolved, guard.ModelTier(in.Model))
	_ = h.recorder().Record(ctx, run.Event{Kind: run.KindLabel, Agent: id, Name: name, Role: role, Wave: wave, Model: model})
}

// ---- lead tokens (Stop) ----

// recordLeadTokens records a `lead-tokens` note with the lead's token total
// since the run started, when it grew. The Stop decision never depends on it.
func (h *handler) recordLeadTokens(ctx context.Context, p payload, st run.State) {
	if h.platform != "claude" || p.TranscriptPath == "" || st.ID == "" || st.Ended {
		return
	}
	total := leadTokens(p.TranscriptPath, st.Started, maxLeadTranscriptBytes)
	if total <= st.LeadTokens {
		return
	}
	_ = h.recorder().Record(ctx, run.Event{Kind: run.KindNote, Name: run.NoteLeadTokens, Tokens: total})
}

// leadLine is a transcript line as far as the lead's accounting reads it.
type leadLine struct {
	Type        string          `json:"type"`
	IsSidechain bool            `json:"isSidechain"`
	Timestamp   string          `json:"timestamp"`
	Message     json.RawMessage `json:"message"`
}

// parseLead decodes an assistant, non-sidechain transcript line that carries
// usage (the lead's own turns; subagent turns are sidechains).
func parseLead(line []byte) (id string, u *usage, at time.Time, ok bool) {
	if !bytes.Contains(line, usageKey) {
		return "", nil, time.Time{}, false
	}
	var l leadLine
	if json.Unmarshal(line, &l) != nil || l.Type != "assistant" || l.IsSidechain {
		return "", nil, time.Time{}, false
	}
	var m transcriptMessage
	if len(l.Message) == 0 || l.Message[0] != '{' || json.Unmarshal(l.Message, &m) != nil || m.Usage == nil {
		return "", nil, time.Time{}, false
	}
	at, _ = time.Parse(time.RFC3339Nano, l.Timestamp)
	return m.ID, m.Usage, at, true
}

// sumLead sums the usage (input, output, cache creation, cache read) of the
// lead's assistant lines stamped at or after since (zero = all), counting
// each message id once at its highest total, as sumTranscript does. Lines
// with no usable timestamp are skipped when since is set.
func sumLead(r io.Reader, since time.Time) (int64, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var total int64
	byID := map[string]int64{}
	for {
		line, err := br.ReadBytes('\n')
		if id, u, at, ok := parseLead(line); ok && (since.IsZero() || !at.IsZero() && !at.Before(since)) {
			if id == "" {
				total += u.sum()
			} else {
				byID[id] = max(byID[id], u.sum())
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	for _, n := range byID {
		total += n
	}
	return total, nil
}

// openTail opens path positioned at its last limit bytes. cut reports that
// the position is mid-file, so the first (partial) line must be dropped.
func openTail(path string, limit int64) (f *os.File, cut bool, err error) {
	f, err = os.Open(path)
	if err != nil {
		return nil, false, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, false, err
	}
	if fi.Size() > limit {
		if _, err := f.Seek(fi.Size()-limit, io.SeekStart); err != nil {
			f.Close()
			return nil, false, err
		}
		cut = true
	}
	return f, cut, nil
}

// tailReader returns a reader over the tail of path with a partial first line
// removed. Close it with the returned func.
func tailReader(path string, limit int64) (io.Reader, func(), error) {
	f, cut, err := openTail(path, limit)
	if err != nil {
		return nil, nil, err
	}
	br := bufio.NewReaderSize(f, 1<<20)
	if cut {
		for { // drop the partial first line
			_, err := br.ReadSlice('\n')
			if !errors.Is(err, bufio.ErrBufferFull) {
				break
			}
		}
	}
	return br, func() { f.Close() }, nil
}

// leadTokens totals the lead's tokens in the transcript at path since the
// run started. It fails open: no path, an unreadable file or a read error is 0.
func leadTokens(path string, since time.Time, limit int64) int64 {
	r, closeFn, err := tailReader(path, limit)
	if err != nil {
		return 0
	}
	defer closeFn()
	n, err := sumLead(r, since)
	if err != nil {
		return 0
	}
	return n
}

// ---- context hint (PostToolUse Bash, `loops run start`) ----

// lastContext returns the context size of the lead's last assistant message
// in the transcript's tail: input plus cache read plus cache creation.
func lastContext(path string) int64 {
	r, closeFn, err := tailReader(path, contextTailBytes)
	if err != nil {
		return 0
	}
	defer closeFn()
	br := bufio.NewReaderSize(r, 1<<20)
	var ctx int64
	for {
		line, err := br.ReadBytes('\n')
		if _, u, _, ok := parseLead(line); ok {
			if n := u.Input + u.CacheRead + u.CacheCreate; n > 0 {
				ctx = n
			}
		}
		if err != nil {
			break
		}
	}
	return ctx
}

// contextHint tells the lead, once, when a run starts in a session that
// already holds a lot of context: the lead re-reads it on every turn.
func (h *handler) contextHint(_ context.Context, d dialect, p payload, in toolInput, out io.Writer) {
	if h.platform != "claude" || p.TranscriptPath == "" || !runStartCmd.MatchString(in.Command) {
		return
	}
	n := lastContext(p.TranscriptPath)
	if n <= contextHintTokens {
		return
	}
	d.postToolContext(out, fmt.Sprintf("loops: this session already holds ~%dk tokens of context, and the lead re-reads it on every turn of this run. "+
		"Tell the user once, in one line: run /clear and say \"continue the loops run\" (the run re-anchors from its brief), or start the next run in a fresh session.", n/1000))
}
