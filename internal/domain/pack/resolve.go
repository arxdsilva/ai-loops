package pack

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

// View is read access to one candidate root's files (slash paths relative
// to that root). The caller implements it; the pack package does no I/O.
type View interface {
	// Exists reports whether rel is an existing file or directory.
	Exists(rel string) bool
	// Read returns rel's content (ok=false when unreadable).
	Read(rel string) (string, bool)
	// List returns the entries directly under dir ("." = the root).
	List(dir string) []string
}

// Matches reports whether every marker matches in v.
func Matches(ms []Marker, v View) bool {
	if len(ms) == 0 {
		return false
	}
	for _, m := range ms {
		if !matchOne(m, v) {
			return false
		}
	}
	return true
}

// AnyMatches reports whether at least one marker matches in v.
func AnyMatches(ms []Marker, v View) bool {
	for _, m := range ms {
		if matchOne(m, v) {
			return true
		}
	}
	return false
}

func matchOne(m Marker, v View) bool {
	if m.File != "" {
		if !v.Exists(m.File) {
			return false
		}
		if m.Contains == "" {
			return true
		}
		s, ok := v.Read(m.File)
		return ok && strings.Contains(s, m.Contains)
	}
	dir := path.Dir(m.Glob)
	if strings.Contains(dir, "*") {
		dir = "."
	}
	for _, e := range v.List(dir) {
		rel := e
		if dir != "." {
			rel = dir + "/" + e
		}
		if glob.Match(m.Glob, rel) {
			return true
		}
	}
	return false
}

// Detected reports whether the pack applies to the root v describes: any
// detect marker matches and no detect_not marker does. Detect markers are
// alternatives (go.mod, or pom.xml, or build.gradle.kts …).
func (p Pack) Detected(v View) bool {
	return AnyMatches(p.Detect, v) && !AnyMatches(p.DetectNot, v)
}

// Resolve applies the first variant whose markers all match (in order) and
// returns the effective pack; without a match it returns p unchanged.
func (p Pack) Resolve(v View) Pack {
	for _, vr := range p.Variants {
		if Matches(vr.Detect, v) {
			return p.apply(vr)
		}
	}
	return p
}

// WithVariant returns the pack with the named variant applied.
func (p Pack) WithVariant(name string) (Pack, error) {
	for _, vr := range p.Variants {
		if vr.Name == name {
			return p.apply(vr), nil
		}
	}
	return Pack{}, fmt.Errorf("pack %s: no variant %q", p.ID, name)
}

func (p Pack) apply(vr Variant) Pack {
	out := p
	out.Variant = vr.Name
	out.Checks = mergeChecks(p.Checks, vr.Checks)
	if len(vr.TestCommands) > 0 {
		out.TestCommands = vr.TestCommands
	}
	if vr.Coverage != nil {
		out.Coverage = mergeCoverage(p.Coverage, *vr.Coverage)
	}
	if vr.PostEdit != nil {
		out.PostEdit = vr.PostEdit
	}
	if vr.CacheEnv != nil {
		out.CacheEnv = vr.CacheEnv
	}
	if vr.CacheArgv != nil {
		out.CacheArgv = vr.CacheArgv
	}
	if vr.Env != nil {
		out.Env = vr.Env
	}
	if vr.Artifacts != nil {
		out.Artifacts = vr.Artifacts
	}
	if vr.Race != nil {
		out.Race = *vr.Race
	}
	return out
}

// mergeChecks overlays variant checks onto base by name: set fields of the
// variant check replace the base's; new names are appended.
func mergeChecks(base, over []Check) []Check {
	out := append([]Check{}, base...)
	for _, o := range over {
		i := -1
		for j := range out {
			if out[j].Name == o.Name {
				i = j
				break
			}
		}
		if i < 0 {
			out = append(out, o)
			continue
		}
		out[i] = overlayCheck(out[i], o)
	}
	return out
}

func overlayCheck(b, o Check) Check {
	if o.Kind != "" {
		b.Kind = o.Kind
	}
	if len(o.Argv) > 0 {
		b.Argv = o.Argv
	}
	if o.Tool != "" {
		b.Tool = o.Tool
	}
	if o.Install != "" {
		b.Install = o.Install
	}
	if o.Required {
		b.Required = true
	}
	if len(o.WhenFile) > 0 {
		b.WhenFile = o.WhenFile
	}
	if len(o.FallbackArgv) > 0 {
		b.FallbackArgv = o.FallbackArgv
	}
	if o.Env != nil {
		b.Env = o.Env
	}
	if len(o.CoverageArgs) > 0 {
		b.CoverageArgs = o.CoverageArgs
	}
	if o.FailRegex != "" {
		b.FailRegex = o.FailRegex
	}
	if o.ExitCodes != nil {
		b.ExitCodes = o.ExitCodes
	}
	if o.Repeat != 0 {
		b.Repeat = o.Repeat
	}
	if o.Note != "" {
		b.Note = o.Note
	}
	if o.Internal != "" {
		b.Internal = o.Internal
	}
	if o.Default != "" {
		b.Default = o.Default
	}
	if o.Skippable != nil {
		b.Skippable = o.Skippable
	}
	if o.SkipOn != nil {
		b.SkipOn = o.SkipOn
	}
	if o.SignalRegex != "" {
		b.SignalRegex = o.SignalRegex
	}
	if o.FailsOn != "" {
		b.FailsOn = o.FailsOn
	}
	if o.OutputExclude != nil {
		b.OutputExclude = o.OutputExclude
	}
	if o.EnvRequired != nil {
		b.EnvRequired = o.EnvRequired
	}
	return b
}

func mergeCoverage(b, o Coverage) Coverage {
	if o.Format != "" {
		b.Format = o.Format
	}
	if o.Path != "" {
		b.Path = o.Path
	}
	if len(o.ProducedBy) > 0 {
		b.ProducedBy = o.ProducedBy
	}
	if o.Produce != nil {
		b.Produce = o.Produce
	}
	if o.Exclude != nil {
		b.Exclude = o.Exclude
	}
	if o.SourceRoots != nil {
		b.SourceRoots = o.SourceRoots
	}
	return b
}

// IsSource reports whether rel (a slash path relative to the pack root) is
// a source file of the pack: it matches source_globs and not test_globs.
func (p Pack) IsSource(rel string) bool {
	return glob.Any(p.SourceGlobs, rel) && !glob.Any(p.TestGlobs, rel)
}

// IsTest reports whether rel matches the pack's test_globs.
func (p Pack) IsTest(rel string) bool { return glob.Any(p.TestGlobs, rel) }

// IsGenerated reports whether rel is generated: a generated glob matches, or
// one of the first lines of head (the file's opening) matches header_regex.
func (p Pack) IsGenerated(rel, head string) bool {
	if glob.Any(p.Generated.Globs, rel) {
		return true
	}
	if p.Generated.HeaderRegex == "" || head == "" {
		return false
	}
	re, err := regexp.Compile(p.Generated.HeaderRegex)
	if err != nil {
		return false
	}
	for i, l := range strings.Split(head, "\n") {
		if i >= 20 {
			break
		}
		if re.MatchString(strings.TrimRight(l, "\r")) {
			return true
		}
	}
	return false
}

// IsTestCommand reports whether argv starts with one of the pack's test
// command prefixes.
func (p Pack) IsTestCommand(argv []string) bool {
	for _, tc := range p.TestCommands {
		if hasPrefix(argv, tc.ArgvPrefix) {
			return true
		}
	}
	return false
}

func hasPrefix(argv, prefix []string) bool {
	if len(prefix) == 0 || len(argv) < len(prefix) {
		return false
	}
	for i, x := range prefix {
		if argv[i] != x {
			return false
		}
	}
	return true
}
