// Package htmlreport renders a run as a self-contained offline HTML page.
package htmlreport

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arxdsilva/ai-loops/internal/domain/run"
)

//go:embed assets/*
var assets embed.FS

// stepCap bounds the step log rows rendered into the page.
const stepCap = 3000

// maxDocBytes bounds each inlined document.
const maxDocBytes = 256 << 10

var tmpl = template.Must(template.New("report.html.tmpl").Funcs(template.FuncMap{
	"statusClass": statusClass,
}).ParseFS(assets, "assets/report.html.tmpl"))

// renderer renders reports.
type renderer struct{}

// New returns a renderer.
func New() *renderer { return &renderer{} }

type phaseView struct {
	Index     int
	Title     string
	Class     string
	Time      string
	Reentries int
}

type laneView struct {
	Name, Role, Model, QA, QAClass, Class, Span, Tokens string
	Wave, Steps, QARounds                               int
	Left, Width                                         string
}

type overrideView struct{ LoopsRule, ProjectRule, Note, Who, Phase string }

type limitView struct {
	Phase, Used, Max, Agents, Roles string
	Over                            bool
}

type budgetView struct{ Time, Phase, Role, Limit, Reason string }

type proofView struct{ Ref, URL, Note string }

type acView struct {
	Name, Status string
	Proofs       []proofView
}

type gateHist struct {
	Time, Name, Note string
	OK               bool
	MS               int64
}

type stepView struct {
	Time, Lane, Tool, Target string
	OK                       bool
	MS                       int64
}

type noteView struct{ Time, Name, Lane, Target, Note string }

type docView struct{ Title, File, Text string }

type view struct {
	CSS                                              template.CSS
	JS                                               template.JS
	S                                                run.State
	ID, Title, Ticket, Source, Outcome, OutcomeClass string
	TicketURL, PRURL, PR                             string
	Duration                                         string
	Phases                                           []phaseView
	Lanes                                            []laneView
	ACs                                              []acView
	GateHistory                                      []gateHist
	Steps                                            []stepView
	StepsTrunc                                       bool
	StepCap                                          int
	LaneOpts                                         []string
	Denials, Memory                                  []noteView
	Docs                                             []docView
	Overrides                                        []overrideView
	Limits                                           []limitView
	GlobalMaxAgents                                  int
	Budgets                                          []budgetView
}

// Render writes the report for state/events; dir is the run directory (for
// linking and inlining ticket.md, plan, proofs, reflection).
func (r *renderer) Render(w io.Writer, s run.State, events []run.Event, dir string) error {
	css, err := assets.ReadFile("assets/style.css")
	if err != nil {
		return err
	}
	js, err := assets.ReadFile("assets/app.js")
	if err != nil {
		return err
	}
	v := buildView(s, events, dir)
	v.CSS, v.JS = template.CSS(css), template.JS(js)
	return tmpl.Execute(w, v)
}

func statusClass(st string) string {
	switch st {
	case run.StatusVerified, run.StatusApprove:
		return "ok"
	case run.StatusFailed, run.StatusChanges:
		return "bad"
	}
	return ""
}

func fmtDur(d time.Duration) string {
	switch {
	case d < 0:
		return ""
	case d < time.Second:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("15:04:05")
}

func webURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return s
}

// safeRel returns ref when it is a plain relative path inside the run dir.
func safeRel(ref string) string {
	if ref == "" || strings.Contains(ref, "://") || strings.ContainsAny(ref, "\\\x00?#") {
		return ""
	}
	c := path.Clean(ref)
	if path.IsAbs(c) || c == ".." || strings.HasPrefix(c, "../") {
		return ""
	}
	return c
}

func buildView(s run.State, events []run.Event, dir string) *view {
	v := &view{S: s, ID: s.ID, Title: s.Title, Ticket: s.Ticket, Source: s.Source, PR: s.PR, StepCap: stepCap}
	if v.Title == "" {
		v.Title = "(untitled run)"
	}
	v.TicketURL = webURL(s.Ticket)
	v.PRURL = webURL(s.PR)
	switch {
	case s.Outcome != "":
		v.Outcome = s.Outcome
	case s.Ended:
		v.Outcome = "ended"
	default:
		v.Outcome = "in progress"
	}
	switch v.Outcome {
	case run.OutcomeDone:
		v.OutcomeClass = "ok"
	case run.OutcomeFailed, run.OutcomeAbandoned:
		v.OutcomeClass = "bad"
	default:
		v.OutcomeClass = "run"
	}
	if !s.Started.IsZero() && !s.Updated.Before(s.Started) {
		v.Duration = fmtDur(s.Updated.Sub(s.Started))
	}
	v.Phases = phaseViews(s)
	v.Lanes = laneViews(s)
	v.ACs = acViews(s, dir)
	laneName := map[string]string{}
	for _, l := range s.Lanes {
		laneName[l.Agent] = laneTitle(l)
	}
	lanesSeen := map[string]bool{}
	for _, e := range events {
		lane := func() string {
			if e.Agent == "" {
				return "lead"
			}
			if n, ok := laneName[e.Agent]; ok {
				return n
			}
			if e.Role != "" {
				return e.Role
			}
			return e.Agent
		}
		switch e.Kind {
		case run.KindGate:
			v.GateHistory = append(v.GateHistory, gateHist{fmtTime(e.Time), e.Name, e.Note, e.OK != nil && *e.OK, e.DurationMS})
		case run.KindStep:
			if len(v.Steps) >= stepCap {
				v.StepsTrunc = true
				continue
			}
			l := lane()
			if !lanesSeen[l] {
				lanesSeen[l] = true
				v.LaneOpts = append(v.LaneOpts, l)
			}
			v.Steps = append(v.Steps, stepView{fmtTime(e.Time), l, e.Name, e.Target, e.OK != nil && *e.OK, e.DurationMS})
		case run.KindGuardDeny:
			v.Denials = append(v.Denials, noteView{fmtTime(e.Time), e.Name, lane(), e.Target, e.Note})
		case run.KindMemory:
			v.Memory = append(v.Memory, noteView{fmtTime(e.Time), e.Name, lane(), e.Target, e.Note})
		}
	}
	v.Docs = docViews(dir)
	v.Overrides, v.Limits, v.Budgets = limitViews(s)
	v.GlobalMaxAgents = s.Limits.MaxAgents
	return v
}

// fmtTokens renders a token count in k/M units ("12k", "1.2M").
func fmtTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		t := n / 100_000 // truncated tenths of a million
		if t%10 == 0 {
			return strconv.FormatInt(t/10, 10) + "M"
		}
		return strconv.FormatInt(t/10, 10) + "." + strconv.FormatInt(t%10, 10) + "M"
	case n >= 1000:
		return strconv.FormatInt(n/1000, 10) + "k"
	}
	return strconv.FormatInt(n, 10)
}

func limitViews(s run.State) ([]overrideView, []limitView, []budgetView) {
	var ov []overrideView
	for _, o := range s.Overrides {
		who := o.Role
		if who == "" {
			who = o.Agent
		}
		ov = append(ov, overrideView{o.LoopsRule, o.ProjectRule, o.Note, who, string(o.Phase)})
	}
	var lv []limitView
	for _, p := range run.Phases {
		pl, ok := s.Limits.Phases[p]
		if !ok {
			continue
		}
		v := limitView{Phase: p.Title(), Used: fmtTokens(s.PhaseTokens[p]), Roles: strings.Join(pl.Roles, ", ")}
		if pl.MaxTokens > 0 {
			v.Max = fmtTokens(pl.MaxTokens)
			v.Over = s.PhaseTokens[p] > pl.MaxTokens
		}
		if pl.MaxAgents > 0 {
			v.Agents = strconv.Itoa(pl.MaxAgents)
		}
		lv = append(lv, v)
	}
	var bv []budgetView
	for _, b := range s.Budgets {
		bv = append(bv, budgetView{fmtTime(b.At), b.Phase.Title(), b.Role, b.Limit, b.Reason})
	}
	return ov, lv, bv
}

func laneTitle(l run.Lane) string {
	switch {
	case l.Label != "":
		return l.Label
	case l.Role != "":
		return l.Role
	}
	return l.Agent
}

func phaseViews(s run.State) []phaseView {
	end := s.Updated
	if end.IsZero() && len(s.PhaseLog) > 0 {
		end = s.PhaseLog[len(s.PhaseLog)-1].At
	}
	spent := map[run.Phase]time.Duration{}
	count := map[run.Phase]int{}
	for i, pe := range s.PhaseLog {
		count[pe.Phase]++
		next := end
		if i+1 < len(s.PhaseLog) {
			next = s.PhaseLog[i+1].At
		}
		if d := next.Sub(pe.At); d > 0 {
			spent[pe.Phase] += d
		}
	}
	cur := s.Phase.Index()
	out := make([]phaseView, 0, len(run.Phases))
	for i, p := range run.Phases {
		pv := phaseView{Index: i, Title: p.Title(), Class: "todo"}
		switch {
		case count[p] > 0 && i == cur && !s.Ended:
			pv.Class = "cur"
		case count[p] > 0:
			pv.Class = "done"
		}
		if d, ok := spent[p]; ok {
			pv.Time = fmtDur(d)
		}
		if count[p] > 1 {
			pv.Reentries = count[p] - 1
		}
		out = append(out, pv)
	}
	return out
}

func laneViews(s run.State) []laneView {
	lo, hi := s.Started, s.Updated
	for _, l := range s.Lanes {
		if lo.IsZero() || (!l.Started.IsZero() && l.Started.Before(lo)) {
			lo = l.Started
		}
		if l.Stopped.After(hi) {
			hi = l.Stopped
		}
		if l.Started.After(hi) {
			hi = l.Started
		}
	}
	total := hi.Sub(lo)
	out := make([]laneView, 0, len(s.Lanes))
	for _, l := range s.Lanes {
		lv := laneView{Name: laneTitle(l), Role: l.Role, Model: l.Model, Wave: l.Wave, Steps: l.Steps,
			QA: l.QA, QAClass: statusClass(l.QA), QARounds: l.QARounds}
		if l.Tokens > 0 {
			lv.Tokens = fmtTokens(l.Tokens)
		}
		stop := l.Stopped
		if stop.IsZero() || stop.Before(l.Started) {
			stop = hi
		}
		switch {
		case l.Running:
			lv.Class = "run"
		case l.OK != nil && !*l.OK:
			lv.Class = "bad"
		case l.OK != nil:
			lv.Class = "ok"
		}
		left, width := 0.0, 100.0
		if total > 0 && !l.Started.IsZero() {
			left = float64(l.Started.Sub(lo)) / float64(total) * 100
			width = float64(stop.Sub(l.Started)) / float64(total) * 100
		}
		if left < 0 {
			left = 0
		}
		if left > 99 {
			left = 99
		}
		if width < 0.5 {
			width = 0.5
		}
		if left+width > 100 {
			width = 100 - left
		}
		lv.Left = strconv.FormatFloat(left, 'f', 2, 64)
		lv.Width = strconv.FormatFloat(width, 'f', 2, 64)
		lv.Span = fmtDur(stop.Sub(l.Started))
		out = append(out, lv)
	}
	return out
}

func acViews(s run.State, dir string) []acView {
	idx := map[string]*acView{}
	var order []string
	get := func(name string) *acView {
		if a, ok := idx[name]; ok {
			return a
		}
		idx[name] = &acView{Name: name}
		order = append(order, name)
		return idx[name]
	}
	for _, vd := range s.Verdicts {
		// plan verdicts and "<agent>:C<n>" claims are not acceptance criteria
		if vd.Phase == run.PhasePlan || vd.Phase == run.PhaseQA || strings.Contains(vd.Name, ":") {
			continue
		}
		get(vd.Name).Status = vd.Status
	}
	for _, p := range s.Proofs {
		a := get(p.Name)
		pv := proofView{Ref: p.Ref, Note: p.Note}
		if rel := safeRel(p.Ref); rel != "" {
			pv.URL = (&url.URL{Path: rel}).String()
		}
		a.Proofs = append(a.Proofs, pv)
	}
	out := make([]acView, 0, len(order))
	for _, n := range order {
		out = append(out, *idx[n])
	}
	return out
}

func readCapped(p string) (string, bool) {
	f, err := os.Open(p)
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf := make([]byte, maxDocBytes+1)
	n, _ := io.ReadFull(f, buf)
	text := string(buf[:n])
	if n > maxDocBytes {
		text = string(buf[:maxDocBytes]) + "\n… (truncated)"
	}
	return strings.ToValidUTF8(text, "�"), true
}

func docViews(dir string) []docView {
	if dir == "" {
		return nil
	}
	var docs []docView
	add := func(title, file string) {
		if t, ok := readCapped(filepath.Join(dir, file)); ok {
			docs = append(docs, docView{Title: title, File: file, Text: t})
		}
	}
	add("Ticket", "ticket.md")
	if f := latestPlan(dir); f != "" {
		add("Plan", f)
	}
	add("Reflection", "reflection.md")
	add("Pull request", "pr.md")
	return docs
}

func latestPlan(dir string) string {
	ms, _ := filepath.Glob(filepath.Join(dir, "plan.v*.md"))
	best, bestN := "", -1
	for _, m := range ms {
		base := filepath.Base(m)
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(base, "plan.v"), ".md"))
		if err == nil && n > bestN {
			best, bestN = base, n
		}
	}
	return best
}
