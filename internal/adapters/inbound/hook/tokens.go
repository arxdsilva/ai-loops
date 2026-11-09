package hook

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// maxTranscriptBytes caps how much of an agent transcript is read (WORKFLOW §4a).
const maxTranscriptBytes = 50 << 20

// agentTranscript is the stopped subagent's own transcript path. Only the
// agent-specific fields are read: transcript_path on a SubagentStop is the
// lead's session, and counting it would charge the lead's tokens to the agent.
func (p payload) agentTranscript() string {
	if p.AgentTranscriptPath != "" {
		return p.AgentTranscriptPath
	}
	return p.AgentTranscriptPathCamel
}

// transcriptTokens sums the usage tokens recorded in the first limit bytes
// of the JSONL transcript at path. It fails open: no path, an unreadable
// file or a read error is 0.
func transcriptTokens(path string, limit int64) int64 {
	if path == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n, err := sumTranscript(io.LimitReader(f, limit))
	if err != nil {
		return 0
	}
	return n
}

// usage is a token usage object in any of the shapes seen in transcripts:
// Anthropic (input, output, cache creation, cache read), OpenAI (prompt,
// completion) and a bare total.
type usage struct {
	Input       int64 `json:"input_tokens"`
	Output      int64 `json:"output_tokens"`
	CacheCreate int64 `json:"cache_creation_input_tokens"`
	CacheRead   int64 `json:"cache_read_input_tokens"`
	Prompt      int64 `json:"prompt_tokens"`
	Completion  int64 `json:"completion_tokens"`
	Total       int64 `json:"total_tokens"`
}

func (u *usage) sum() int64 {
	if u == nil {
		return 0
	}
	if n := u.Input + u.Output + u.CacheCreate + u.CacheRead; n > 0 {
		return n
	}
	if n := u.Prompt + u.Completion; n > 0 {
		return n
	}
	return u.Total
}

// transcriptLine is the subset of a transcript line that carries usage.
type transcriptLine struct {
	Message json.RawMessage `json:"message"` // object on Claude, sometimes a string elsewhere
	Usage   *usage          `json:"usage"`
	Payload *struct {
		Type string `json:"type"`
		Info *struct {
			Total *usage `json:"total_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

type transcriptMessage struct {
	ID    string `json:"id"`
	Usage *usage `json:"usage"`
}

// sumTranscript sums the tokens of a JSONL transcript:
//   - message.usage (Claude): every input, output and cache field. Claude
//     writes one line per content block, each repeating the message's usage,
//     so a message id is counted once, at its highest total;
//   - top-level usage (other agents);
//   - payload.info.total_token_usage (Codex token_count events): a running
//     total, so the highest one counts.
//
// Lines that are not JSON or carry no usage are skipped; only a read error
// is returned.
func sumTranscript(r io.Reader) (int64, error) {
	br := bufio.NewReader(r)
	var total, cumulative int64
	byID := map[string]int64{}
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			total += lineTokens(line, byID, &cumulative)
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
	return total + cumulative, nil
}

// lineTokens returns the tokens line adds directly, recording per-message
// totals in byID and running totals in cumulative.
func lineTokens(line []byte, byID map[string]int64, cumulative *int64) int64 {
	var l transcriptLine
	if json.Unmarshal(line, &l) != nil {
		return 0
	}
	if p := l.Payload; p != nil && p.Info != nil && p.Info.Total != nil {
		*cumulative = max(*cumulative, p.Info.Total.sum())
	}
	var n int64
	var m transcriptMessage
	if len(l.Message) > 0 && l.Message[0] == '{' && json.Unmarshal(l.Message, &m) == nil && m.Usage != nil {
		if m.ID != "" {
			byID[m.ID] = max(byID[m.ID], m.Usage.sum())
		} else {
			n += m.Usage.sum()
		}
	}
	return n + l.Usage.sum()
}
