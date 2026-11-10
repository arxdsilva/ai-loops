// Package archcheck runs the hexagonal + SOLID architecture rules
// (specs/ARCHITECTURE.md, specs/SOLID.md) over a Go module, or over every
// detected language pack's root (research/languages/TOOLING.md §6).
package archcheck

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/application/langpack"
	"github.com/arxdsilva/ai-loops/internal/domain/archrule"
	"github.com/arxdsilva/ai-loops/internal/domain/pack"
)

// Parser turns a module tree into the package model the rules evaluate.
type Parser interface {
	Parse(ctx context.Context, root string) (archrule.Module, error)
}

// Scanner builds the model of a pack whose arch.parser is "regex" from its
// source globs, import regex and resolver.
type Scanner interface {
	Scan(ctx context.Context, root string, p pack.Pack) (archrule.Module, error)
}

// Service runs the checks.
type Service struct {
	parser  Parser
	scanner Scanner
	packs   []langpack.Detected
}

// New builds a Service.
func New(p Parser) *Service { return &Service{parser: p} }

// WithScanner sets the scanner for regex-parser packs.
func (s *Service) WithScanner(sc Scanner) *Service { s.scanner = sc; return s }

// WithPacks makes Check run CheckPacks over ds (nil keeps the Go-only check).
func (s *Service) WithPacks(ds []langpack.Detected) *Service { s.packs = ds; return s }

// Check parses root and evaluates every rule with the given layer layout.
// With packs set (WithPacks), it is CheckPacks without the notes.
func (s *Service) Check(ctx context.Context, root string, layers archrule.Layers) (archrule.Report, error) {
	if len(s.packs) > 0 {
		res, err := s.CheckPacks(ctx, root, layers, s.packs)
		return res.Report, err
	}
	m, err := s.parser.Parse(ctx, root)
	if err != nil {
		return archrule.Report{}, err
	}
	return archrule.Evaluate(m, layers), nil
}

// Result is the outcome of CheckPacks.
type Result struct {
	Report archrule.Report `json:"report"`
	// Notes has one line per pack loops does not parse (arch.parser none).
	Notes []string `json:"notes,omitempty"`
}

// CheckPacks checks each detected pack at its root (relative to root) with
// the pack's parser and rule data, and merges the findings. File paths
// become relative to root; with several packs each message starts with
// "<id>@<root>: ". A pack with arch.parser "none" passes with a note: its
// native check runs in the gate.
//
// Layers per pack: the config layers apply when the pack sits at the
// project root, or to the configured entries under the pack's root (taken
// relative to it; "**/" globs apply everywhere). Otherwise a Go pack uses
// archrule.DefaultLayers and any other pack archrule.ConventionalLayers of
// its resolver (layer directory names anywhere in the path).
func (s *Service) CheckPacks(ctx context.Context, root string, layers archrule.Layers, ds []langpack.Detected) (Result, error) {
	res := Result{Report: archrule.Report{Findings: []archrule.Finding{}}}
	var mods []string
	for _, d := range ds {
		rep, note, err := s.checkPack(ctx, root, layers, d)
		if err != nil {
			return Result{}, fmt.Errorf("%s@%s: %w", d.ID, d.Root, err)
		}
		if note != "" {
			res.Notes = append(res.Notes, note)
			continue
		}
		mods = append(mods, d.ID+"@"+d.Root)
		if len(ds) == 1 {
			res.Report.Module = rep.Module
		}
		for _, f := range rep.Findings {
			f.File = path.Join(d.Root, f.File)
			if len(ds) > 1 {
				f.Msg = d.ID + "@" + d.Root + ": " + f.Msg
			}
			res.Report.Findings = append(res.Report.Findings, f)
		}
	}
	if len(ds) > 1 {
		res.Report.Module = strings.Join(mods, ", ")
	}
	return res, nil
}

func (s *Service) checkPack(ctx context.Context, root string, layers archrule.Layers, d langpack.Detected) (archrule.Report, string, error) {
	dir := filepath.Join(root, filepath.FromSlash(d.Root))
	a := d.Pack.Arch
	switch a.Parser {
	case pack.ParserGo, "":
		m, err := s.parser.Parse(ctx, dir)
		if err != nil {
			return archrule.Report{}, "", err
		}
		l, _ := PackLayers(layers, d.Root)
		return archrule.EvaluateWith(m, l, GoPackRules(a)), "", nil
	case pack.ParserRegex:
		if s.scanner == nil {
			return archrule.Report{}, "", fmt.Errorf("arch.parser regex needs an import scanner (archcheck.WithScanner)")
		}
		m, err := s.scanner.Scan(ctx, dir, d.Pack)
		if err != nil {
			return archrule.Report{}, "", err
		}
		l, ok := PackLayers(layers, d.Root)
		if !ok {
			l = archrule.ConventionalLayers(a.Resolver)
		}
		return archrule.EvaluateWith(m, l, PackRules(a)), "", nil
	case pack.ParserNone:
		if a.NativeCheck == "" {
			return archrule.Report{}, fmt.Sprintf("arch: %s has no built-in parser and no native check", d.ID), nil
		}
		return archrule.Report{}, fmt.Sprintf("arch: %s delegates to its native check %s (run by loops gate)", d.ID, a.NativeCheck), nil
	}
	return archrule.Report{}, "", fmt.Errorf("unknown arch.parser %q", a.Parser)
}

// PackRules is a non-Go pack's rule data, taken as given: an empty list
// means none (no third-party restriction unless the pack names layers).
func PackRules(a pack.Arch) archrule.Rules {
	return archrule.Rules{
		ForbiddenInDomain:     a.ForbiddenInDomain,
		ImpureCalls:           a.ImpureCalls,
		ThirdPartyForbiddenIn: a.ThirdPartyForbiddenIn,
		Limits:                a.SizeWarnings,
	}
}

// GoPackRules is the Go pack's rule data; an empty field keeps today's Go
// value (archrule.GoRules).
func GoPackRules(a pack.Arch) archrule.Rules {
	r, def := PackRules(a), archrule.GoRules()
	if len(r.ForbiddenInDomain) == 0 {
		r.ForbiddenInDomain = def.ForbiddenInDomain
	}
	if len(r.ImpureCalls) == 0 {
		r.ImpureCalls = def.ImpureCalls
	}
	if len(r.ThirdPartyForbiddenIn) == 0 {
		r.ThirdPartyForbiddenIn = def.ThirdPartyForbiddenIn
	}
	return r
}

// PackLayers returns the config layers that apply to a pack at packRoot
// and whether any do. At the project root they all apply unchanged.
// Elsewhere an entry applies when it lies under packRoot (it is made
// relative to it) or starts with "**/".
func PackLayers(l archrule.Layers, packRoot string) (archrule.Layers, bool) {
	packRoot = path.Clean(packRoot)
	if packRoot == "." {
		return l, !layersZero(l)
	}
	hit := false
	pick := func(in []string) []string {
		var out []string
		for _, p := range in {
			p = strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, "\\", "/")), "./")
			switch {
			case strings.HasPrefix(p, "**/"):
				out = append(out, p)
			case strings.HasPrefix(p, packRoot+"/"):
				out = append(out, strings.TrimPrefix(p, packRoot+"/"))
			default:
				continue
			}
			hit = true
		}
		return out
	}
	out := archrule.Layers{
		Domain: pick(l.Domain), Application: pick(l.Application), Ports: pick(l.Ports),
		Inbound: pick(l.Inbound), Outbound: pick(l.Outbound), Platform: pick(l.Platform),
		Compose: pick(l.Compose), Cmd: pick(l.Cmd), Pkg: pick(l.Pkg), Adapters: pick(l.Adapters),
	}
	return out, hit
}

func layersZero(l archrule.Layers) bool {
	return len(l.Domain)+len(l.Application)+len(l.Ports)+len(l.Inbound)+len(l.Outbound)+
		len(l.Platform)+len(l.Compose)+len(l.Cmd)+len(l.Pkg)+len(l.Adapters) == 0
}

// Gate adapts the checker to the gate's Arch port (gate.Arch):
// Check(ctx, root) returns the error findings, one "file:line rule msg" line
// each, and whether the check passed. With Packs it runs CheckPacks.
type Gate struct {
	Service *Service
	Layers  archrule.Layers
	Packs   []langpack.Detected
}

// Check implements gate.Arch.
func (g Gate) Check(ctx context.Context, root string) (string, bool, error) {
	var rep archrule.Report
	var err error
	if len(g.Packs) > 0 {
		var res Result
		res, err = g.Service.CheckPacks(ctx, root, g.Layers, g.Packs)
		rep = res.Report
	} else {
		rep, err = g.Service.Check(ctx, root, g.Layers)
	}
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, f := range rep.Findings {
		if f.Severity == archrule.SeverityError {
			fmt.Fprintf(&b, "%s:%d %s %s\n", f.File, f.Line, f.Rule, f.Msg)
		}
	}
	return b.String(), rep.OK(), nil
}
