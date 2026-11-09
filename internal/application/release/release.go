// Package release cuts a release: it picks the next version from the
// commits since the last tag, rewrites the version file, regenerates the
// platform packages, then commits, tags and (optionally) pushes.
package release

import (
	"context"
	"fmt"
	"strings"

	domain "github.com/arxdsilva/ai-loops/internal/domain/release"
)

// ReleasePaths are committed by a release: the version file and everything
// generated from it.
var ReleasePaths = []string{
	".claude-plugin/plugin.json",
	"platforms",
	".agents/plugins/marketplace.json",
	".cursor-plugin/marketplace.json",
}

// Git is the repository as the release needs it.
type Git interface {
	LastTag(ctx context.Context) (string, error) // "" when there is none
	CommitsSince(ctx context.Context, tag string) ([]string, error)
	HeadTags(ctx context.Context) ([]string, error)
	HeadSubject(ctx context.Context) (string, error)
	// Commit stages paths and commits only them (git add -- paths; git commit -m msg -- paths).
	Commit(ctx context.Context, msg string, paths []string) error
	Tag(ctx context.Context, tag, msg string) error // annotated
	Push(ctx context.Context, ref ...string) error
	Dirty(ctx context.Context) (bool, error)
}

// Files is the version file.
type Files interface {
	ReadVersion() (string, error)
	WriteVersion(v string) error
}

// Options configure one release.
type Options struct {
	Level  string // auto, patch, minor or major
	DryRun bool   // plan only: change nothing (works on a dirty tree)
	Push   bool   // push HEAD and the tag
}

// Result is what Release did (or would do).
type Result struct {
	Skipped bool
	Reason  string // why it was skipped
	DryRun  bool
	Current string
	Next    string
	Level   string // the level applied
	Summary string // e.g. "3 feat, 2 fix"
}

// Service is the release use case.
type Service struct {
	git        Git
	files      Files
	regenerate func() error
}

// New returns a Service; regenerate rebuilds the platform packages from the
// (just rewritten) sources.
func New(g Git, f Files, regenerate func() error) *Service {
	return &Service{git: g, files: f, regenerate: regenerate}
}

// Release runs the flow. It skips (no error) when HEAD is already released
// or there is nothing new, and refuses a dirty tree unless DryRun.
func (s *Service) Release(ctx context.Context, o Options) (Result, error) {
	res := Result{DryRun: o.DryRun}
	last, err := s.git.LastTag(ctx)
	if err != nil {
		return res, err
	}
	commits, err := s.git.CommitsSince(ctx, last)
	if err != nil {
		return res, err
	}
	tags, err := s.git.HeadTags(ctx)
	if err != nil {
		return res, err
	}
	subject, err := s.git.HeadSubject(ctx)
	if err != nil {
		return res, err
	}
	for _, t := range tags {
		if strings.HasPrefix(t, "v") {
			return skip(res, "HEAD is already tagged "+t), nil
		}
	}
	if strings.HasPrefix(subject, "release v") {
		return skip(res, "HEAD is a release commit"), nil
	}
	if len(commits) == 0 {
		return skip(res, "no commits since "+orNone(last)), nil
	}
	dirty, err := s.git.Dirty(ctx)
	if err != nil {
		return res, err
	}
	if dirty && !o.DryRun {
		return res, fmt.Errorf("release: working tree is dirty; commit or stash first (or use --dry-run)")
	}
	if res.Current, err = s.files.ReadVersion(); err != nil {
		return res, err
	}
	if res.Next, res.Level, err = domain.Bump(res.Current, o.Level, commits); err != nil {
		return res, err
	}
	res.Summary = domain.Summarize(commits).String()
	if o.DryRun {
		return res, nil
	}
	tag := "v" + res.Next
	if err := s.files.WriteVersion(res.Next); err != nil {
		return res, err
	}
	if err := s.regenerate(); err != nil {
		return res, fmt.Errorf("release: regenerate packages: %w", err)
	}
	if err := s.git.Commit(ctx, "release "+tag, ReleasePaths); err != nil {
		return res, err
	}
	if err := s.git.Tag(ctx, tag, "loops "+tag); err != nil {
		return res, err
	}
	if o.Push {
		if err := s.git.Push(ctx, "HEAD", tag); err != nil {
			return res, err
		}
	}
	return res, nil
}

func skip(r Result, reason string) Result {
	r.Skipped, r.Reason = true, reason
	return r
}

func orNone(tag string) string {
	if tag == "" {
		return "the beginning"
	}
	return tag
}
