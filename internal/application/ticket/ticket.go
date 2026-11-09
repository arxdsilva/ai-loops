// Package ticket fetches a ticket (GitHub issue, GitLab issue, or free text)
// and normalises it to ticket.md (templates/TICKET.md). Jira and Linear are
// fetched by the lead through their MCP connectors and passed in as text.
package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Source of a ticket.
const (
	SourceGitHub = "github"
	SourceGitLab = "gitlab"
	SourceJira   = "jira"
	SourceLinear = "linear"
	SourceText   = "text"
)

// Issues fetches issues from a forge CLI.
type Issues interface {
	// GitHub runs `gh issue view <ref> --json number,title,body,labels,url`
	// and returns its JSON.
	GitHub(ctx context.Context, ref string) ([]byte, error)
	// GitLab runs `glab issue view <ref> --output json` and returns its JSON.
	GitLab(ctx context.Context, ref string) ([]byte, error)
}

// Ticket is the normalised ticket.
type Ticket struct {
	Source   string
	Ref      string // "#123", URL, "ABC-123", or ""
	Title    string
	Body     string
	URL      string
	Labels   []string
	Criteria []string // acceptance criteria parsed from the body (checklists / "Acceptance criteria" section)
}

// Service fetches tickets.
type Service struct{ issues Issues }

// New builds a Service.
func New(issues Issues) *Service { return &Service{issues: issues} }

var (
	jiraKey     = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	numericRef  = regexp.MustCompile(`^#?\d+$`)
	repoIssue   = regexp.MustCompile(`^[\w.-]+(/[\w.-]+)+#\d+$`)
	acPrefix    = regexp.MustCompile(`(?i)^AC-?\d+\s*[:.)-]\s*`)
	listItem    = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*)$`)
	checkItem   = regexp.MustCompile(`^\s*[-*+]\s+\[[ xX]\]\s+(.*)$`)
	acHeading   = regexp.MustCompile(`(?i)^\s*(?:#{1,6}\s*|\*\*)?(?:acceptance criteria|ac|acs|definition of done)\s*(?:\*\*)?:?\s*$`)
	anyHeading  = regexp.MustCompile(`^\s*#{1,6}\s+\S`)
	boldHeading = regexp.MustCompile(`^\s*\*\*[^*]+\*\*:?\s*$`)
)

// Detect classifies ref: "#123"/"123"/github.com URL → github; gitlab URL or
// "group/proj#12" with a gitlab remote → gitlab; "ABC-123" → jira;
// linear.app URL or "LIN-123"-style with source hint → linear; else text.
func Detect(ref, remoteURL string) string {
	ref = strings.TrimSpace(ref)
	low := strings.ToLower(ref)
	gitlabRemote := strings.Contains(strings.ToLower(remoteURL), "gitlab")
	switch {
	case ref == "":
		return SourceText
	case strings.Contains(low, "linear.app"):
		return SourceLinear
	case strings.Contains(low, "github.com"):
		return SourceGitHub
	case strings.Contains(low, "gitlab"):
		return SourceGitLab
	case jiraKey.MatchString(ref):
		return SourceJira
	case numericRef.MatchString(ref), repoIssue.MatchString(ref):
		if gitlabRemote {
			return SourceGitLab
		}
		return SourceGitHub
	}
	return SourceText
}

// Fetch resolves ref into a Ticket. For jira, linear and text, body is the
// text supplied by the caller (the lead fetched it via MCP or the user typed
// it); Fetch only parses it.
func (s *Service) Fetch(ctx context.Context, source, ref, body string) (Ticket, error) {
	switch source {
	case SourceGitHub, SourceGitLab:
		if strings.TrimSpace(ref) == "" {
			return Ticket{}, fmt.Errorf("ticket: %s needs a ref", source)
		}
		var raw []byte
		var err error
		if source == SourceGitHub {
			raw, err = s.issues.GitHub(ctx, ref)
		} else {
			raw, err = s.issues.GitLab(ctx, ref)
		}
		if err != nil {
			return Ticket{}, err
		}
		t, err := parseForge(raw)
		if err != nil {
			return Ticket{}, fmt.Errorf("ticket: parse %s json: %w", source, err)
		}
		t.Source, t.Ref = source, ref
		t.Criteria = Criteria(t.Body)
		return t, nil
	case SourceJira, SourceLinear, SourceText:
		if strings.TrimSpace(body) == "" {
			return Ticket{}, errors.New("ticket: empty body for " + source + " ticket")
		}
		title, rest := splitTitle(body)
		return Ticket{Source: source, Ref: ref, Title: title, Body: rest, Criteria: Criteria(rest)}, nil
	}
	return Ticket{}, fmt.Errorf("ticket: unknown source %q", source)
}

func parseForge(raw []byte) (Ticket, error) {
	var j struct {
		Title       string            `json:"title"`
		Body        string            `json:"body"`
		Description string            `json:"description"`
		URL         string            `json:"url"`
		WebURL      string            `json:"web_url"`
		Labels      []json.RawMessage `json:"labels"`
	}
	if err := json.Unmarshal(raw, &j); err != nil {
		return Ticket{}, err
	}
	t := Ticket{Title: j.Title, Body: j.Body, URL: j.URL}
	if t.Body == "" {
		t.Body = j.Description
	}
	if t.URL == "" {
		t.URL = j.WebURL
	}
	for _, l := range j.Labels {
		var str string
		var obj struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(l, &str) == nil && str != "" {
			t.Labels = append(t.Labels, str)
		} else if json.Unmarshal(l, &obj) == nil && obj.Name != "" {
			t.Labels = append(t.Labels, obj.Name)
		}
	}
	return t, nil
}

// splitTitle takes the first "# heading" (or else the first non-empty line)
// as the title and returns the remaining text as the body.
func splitTitle(body string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	idx := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "# ") {
			idx = i
			break
		}
	}
	if idx < 0 {
		for i, l := range lines {
			if strings.TrimSpace(l) != "" {
				idx = i
				break
			}
		}
	}
	title := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(lines[idx]), "# "))
	rest := append(append([]string{}, lines[:idx]...), lines[idx+1:]...)
	return title, strings.TrimSpace(strings.Join(rest, "\n"))
}

// Criteria extracts acceptance criteria from body: bullet items under an
// "Acceptance criteria" / "AC" heading, and markdown checklist items anywhere.
func Criteria(body string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(acPrefix.ReplaceAllString(strings.TrimSpace(s), ""))
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	in := false
	for _, l := range strings.Split(body, "\n") {
		switch {
		case acHeading.MatchString(l):
			in = true
		case anyHeading.MatchString(l) || boldHeading.MatchString(l):
			in = false
		case checkItem.MatchString(l):
			add(checkItem.FindStringSubmatch(l)[1])
		case in && listItem.MatchString(l):
			add(listItem.FindStringSubmatch(l)[1])
		}
	}
	return out
}

// Markdown renders t as ticket.md.
func Markdown(t Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t.Title)
	ref := t.Ref
	if ref == "" {
		ref = "-"
	}
	fmt.Fprintf(&b, "Source: %s \u00b7 Ref: %s", t.Source, ref)
	if t.URL != "" {
		fmt.Fprintf(&b, " \u00b7 %s", t.URL)
	}
	b.WriteString("\n")
	if len(t.Labels) > 0 {
		fmt.Fprintf(&b, "Labels: %s\n", strings.Join(t.Labels, ", "))
	}
	b.WriteString("\n## Acceptance criteria\n")
	if len(t.Criteria) == 0 {
		b.WriteString("_none in ticket \u2014 planner derives them_\n")
	}
	for i, c := range t.Criteria {
		fmt.Fprintf(&b, "- AC-%d: %s\n", i+1, c)
	}
	fmt.Fprintf(&b, "\n## Body\n%s\n", t.Body)
	return b.String()
}
