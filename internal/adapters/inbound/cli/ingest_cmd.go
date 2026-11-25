package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/qafile"
	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

// ingestLoopBack is one loop-back in `loops run ingest --json`.
type ingestLoopBack struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Round int    `json:"round"`
	Note  string `json:"note,omitempty"`
}

// ingestJSON is `loops run ingest --json`.
type ingestJSON struct {
	File      string           `json:"file"`
	Ref       string           `json:"ref"`
	Pass      string           `json:"pass"`
	Round     int              `json:"round"`
	Verified  int              `json:"verified"`
	Failed    int              `json:"failed"`
	FailedIDs []string         `json:"failed_ids,omitempty"`
	LoopBacks []ingestLoopBack `json:"loop_backs,omitempty"`
	Recorded  int              `json:"recorded"` // events appended now
	Skipped   int              `json:"skipped"`  // already recorded, left alone
}

// runIngest implements `loops run ingest <file> [--json]`: it records the
// verdicts and loop-backs of a QA file (qa/<agent-id>.r<N>.md, quality.r<N>.md
// or security.r<N>.md, WORKFLOW §2.4) as the lead would with `run verdict` and
// `run loopback`, once: an event already in the log is skipped.
func (a *App) runIngest(ctx context.Context, args []string) int {
	fs := a.newFlagSet("run ingest")
	asJSON := fs.Bool("json", false, "machine-readable")
	pos, err := parseFlagsAnywhere(fs, args)
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		return a.usageErr("usage: loops run ingest <qa file> [--json]")
	}
	body, err := os.ReadFile(pos[0])
	if err != nil {
		fmt.Fprintf(a.Err, "loops run ingest: %v\n", err)
		return 1
	}
	doc, err := qafile.Parse(pos[0], string(body))
	if err != nil {
		return a.usageErr("run ingest: " + err.Error())
	}
	events, err := a.Tracker.Events(ctx, "")
	if err != nil {
		return a.fail("run ingest", err)
	}
	st := run.Fold(events)
	plan := doc.Plan()
	ref := "qa/" + doc.Base
	plan.Verdicts = append(plan.Verdicts, supersede(doc, plan, st)...)

	type vkey struct {
		name, status string
		round        int
	}
	type lkey struct {
		from, to string
		round    int
	}
	haveV, haveL := map[vkey]bool{}, map[lkey]bool{}
	for _, e := range events {
		switch e.Kind {
		case run.KindVerdict:
			haveV[vkey{e.Name, e.Status, e.Round}] = true
		case run.KindLoopBack:
			haveL[lkey{e.From, e.To, e.Round}] = true
		}
	}

	res := ingestJSON{File: pos[0], Ref: ref, Pass: string(doc.Kind), Round: doc.Round}
	for _, v := range plan.Verdicts {
		if v.Status == run.StatusFailed {
			res.Failed++
			res.FailedIDs = append(res.FailedIDs, shortID(v.Name))
		} else {
			res.Verified++
		}
		if haveV[vkey{v.Name, v.Status, doc.Round}] {
			res.Skipped++
			continue
		}
		e := run.Event{Kind: run.KindVerdict, Name: v.Name, Status: v.Status, Round: doc.Round, Ref: ref, Role: v.Role, Phase: st.Phase}
		if err := a.Tracker.Record(ctx, e); err != nil {
			return a.fail("run ingest", err)
		}
		res.Recorded++
	}
	for _, l := range plan.LoopBacks {
		res.LoopBacks = append(res.LoopBacks, ingestLoopBack{From: l.From, To: l.To, Round: doc.Round, Note: l.Note})
		if haveL[lkey{l.From, l.To, doc.Round}] {
			res.Skipped++
			continue
		}
		e := run.Event{Kind: run.KindLoopBack, From: l.From, To: l.To, Round: doc.Round, Note: l.Note, Phase: st.Phase}
		if err := a.Tracker.Record(ctx, e); err != nil {
			return a.fail("run ingest", err)
		}
		res.Recorded++
	}

	if *asJSON {
		b, _ := json.Marshal(res)
		fmt.Fprintln(a.Out, string(b))
		return 0
	}
	fmt.Fprintln(a.Out, res.line())
	return 0
}

// supersede closes, when the pass is clean (no failed verdict in plan), the
// quality: or security: findings an earlier round left open, so the run does
// not keep a failure the pass no longer reports.
func supersede(doc qafile.Doc, plan qafile.Plan, st run.State) []qafile.Verdict {
	if doc.Kind == qafile.KindLane || doc.Verdict != run.StatusVerified {
		return nil
	}
	role := qafile.RoleQuality
	if doc.Kind == qafile.KindSecurity {
		role = qafile.RoleSecurity
	}
	for _, v := range plan.Verdicts {
		if v.Status == run.StatusFailed {
			return nil
		}
	}
	var out []qafile.Verdict
	for _, v := range run.OpenFailures(st) {
		if strings.HasPrefix(v.Name, string(doc.Kind)+":") && v.Status == run.StatusFailed {
			out = append(out, qafile.Verdict{Name: v.Name, Status: run.StatusVerified, Role: role})
		}
	}
	return out
}

// shortID is the claim or finding id of a verdict name ("W1-domain:C3" is
// "C3"; "quality" stays "quality").
func shortID(name string) string {
	if i := strings.LastIndex(name, ":"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// line is the one-line text result.
func (r ingestJSON) line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "ingested %s: %d VERIFIED, %d FAILED", r.Ref, r.Verified, r.Failed)
	if len(r.FailedIDs) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(r.FailedIDs, ", "))
	}
	for i, l := range r.LoopBacks {
		sep := ", "
		if i == 0 {
			sep = "; loopback "
		}
		fmt.Fprintf(&b, "%s%s -> %s r%d", sep, l.From, l.To, l.Round)
	}
	if r.Recorded == 0 && r.Skipped > 0 {
		b.WriteString(" (already recorded)")
	}
	return b.String()
}
