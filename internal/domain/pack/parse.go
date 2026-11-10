package pack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/glob"
)

// Parse decodes and validates one pack. Unknown fields are errors, so a
// typo never silently drops a rule.
func Parse(b []byte) (Pack, error) {
	var p Pack
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Pack{}, fmt.Errorf("pack: %w", err)
	}
	if err := Validate(p); err != nil {
		return Pack{}, err
	}
	return p, nil
}

var idRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// placeholderRe finds {name} placeholders in argv elements.
var placeholderRe = regexp.MustCompile(`\{([a-z_]+)\}`)

// Validate returns every problem in p, joined (nil when the pack is valid).
func Validate(p Pack) error {
	v := &validator{p: p}
	v.top()
	v.markers("detect", p.Detect, true)
	v.markers("detect_not", p.DetectNot, false)
	v.globs("source_globs", p.SourceGlobs, true)
	v.globs("test_globs", p.TestGlobs, true)
	v.globs("generated.globs", p.Generated.Globs, false)
	v.regex("generated.header_regex", p.Generated.HeaderRegex)
	v.checks("checks", p.Checks)
	v.race(p.Race)
	v.coverage(p.Coverage)
	v.cacheEnv(p.CacheEnv)
	v.globs("artifacts", trimDirSlash(p.Artifacts), false)
	v.arch(p.Arch)
	v.placeholders()
	v.variants()
	return errors.Join(v.errs...)
}

type validator struct {
	p    Pack
	errs []error
}

func (v *validator) fail(format string, a ...any) {
	v.errs = append(v.errs, fmt.Errorf("pack %s: "+format, append([]any{v.p.ID}, a...)...))
}

func (v *validator) top() {
	if v.p.Schema != Schema {
		v.fail("schema %d, want %d", v.p.Schema, Schema)
	}
	if !idRe.MatchString(v.p.ID) {
		v.fail("id %q must match %s", v.p.ID, idRe)
	}
	if v.p.Name == "" {
		v.fail("name is empty")
	}
	if v.p.Versions.AsOf == "" {
		v.fail("versions.as_of is empty")
	}
	if len(v.p.TestCommands) == 0 {
		v.fail("test_commands is empty")
	}
	for i, tc := range v.p.TestCommands {
		if len(tc.ArgvPrefix) == 0 {
			v.fail("test_commands[%d].argv_prefix is empty", i)
		}
	}
}

func (v *validator) markers(field string, ms []Marker, need bool) {
	if need && len(ms) == 0 {
		v.fail("%s is empty", field)
	}
	for i, m := range ms {
		if (m.File == "") == (m.Glob == "") {
			v.fail("%s[%d] needs exactly one of file, glob", field, i)
		}
		if m.Contains != "" && m.File == "" {
			v.fail("%s[%d].contains needs file", field, i)
		}
		if m.Glob != "" && !glob.Valid(m.Glob) {
			v.fail("%s[%d].glob %q is malformed", field, i, m.Glob)
		}
	}
}

func (v *validator) globs(field string, gs []string, need bool) {
	if need && len(gs) == 0 {
		v.fail("%s is empty", field)
	}
	for _, g := range gs {
		if !glob.Valid(g) {
			v.fail("%s: malformed glob %q", field, g)
		}
	}
}

func (v *validator) regex(field, re string) {
	if re == "" {
		return
	}
	if _, err := regexp.Compile(re); err != nil {
		v.fail("%s: %v", field, err)
	}
}

func (v *validator) checks(field string, cs []Check) {
	if field == "checks" && len(cs) == 0 {
		v.fail("checks is empty")
	}
	seen := map[string]bool{}
	tests := 0
	for _, c := range cs {
		where := field + "." + c.Name
		if c.Name == "" {
			v.fail("%s: a check has no name", field)
			continue
		}
		if seen[c.Name] {
			v.fail("%s: duplicate check %q", field, c.Name)
		}
		seen[c.Name] = true
		if field == "checks" || c.Kind != "" {
			if !slices.Contains(Kinds, c.Kind) {
				v.fail("%s: kind %q not in %v", where, c.Kind, Kinds)
			}
		}
		if c.Kind == "test" && c.Required {
			tests++
		}
		switch c.Internal {
		case "":
			if field == "checks" && len(c.Argv) == 0 {
				v.fail("%s: argv is empty (or set internal)", where)
			}
		case "arch", "coverage":
			if len(c.Argv) > 0 {
				v.fail("%s: internal check has argv", where)
			}
		default:
			v.fail("%s: internal %q not in [arch coverage]", where, c.Internal)
		}
		if c.Default != "" && c.Default != "on" && c.Default != "off" {
			v.fail("%s: default %q not in [on off]", where, c.Default)
		}
		if c.FailsOn != "" && c.FailsOn != "exit" && c.FailsOn != "output" {
			v.fail("%s: fails_on %q not in [exit output]", where, c.FailsOn)
		}
		for code, out := range c.ExitCodes {
			if out != "pass" && out != "fail" && out != "skip" {
				v.fail("%s: exit_codes[%s] = %q not in [pass fail skip]", where, code, out)
			}
		}
		if len(c.FallbackArgv) > 0 && len(c.WhenFile) == 0 {
			v.fail("%s: fallback_argv needs when_file", where)
		}
		if c.Repeat < 0 {
			v.fail("%s: repeat < 0", where)
		}
		v.regex(where+".fail_regex", c.FailRegex)
		v.regex(where+".signal_regex", c.SignalRegex)
		if c.SkipOn != nil {
			v.regex(where+".skip_on.output_regex", c.SkipOn.OutputRegex)
		}
	}
	if field == "checks" && tests == 0 {
		v.fail("checks: no required check of kind test")
	}
}

func (v *validator) race(r Race) {
	switch r.Mode {
	case RaceFlag:
		if len(r.Flags) == 0 {
			v.fail("race: mode flag needs flags")
		}
	case RaceBuild, RaceNone:
	default:
		v.fail("race.mode %q not in [flag build none]", r.Mode)
	}
	if len(r.Checks) == 0 {
		v.fail("race.checks is empty: name the checks that carry the policy")
	}
	for _, n := range r.Checks {
		c, ok := v.p.CheckByName(n)
		switch {
		case !ok:
			v.fail("race.checks: unknown check %q", n)
		case !c.Required:
			v.fail("race.checks: check %q must be required", n)
		}
	}
	if r.Mode == RaceNone && r.Substitute == "" {
		v.fail("race: mode none needs a substitute description")
	}
}

func (v *validator) coverage(c Coverage) {
	if !slices.Contains(Formats, c.Format) {
		v.fail("coverage.format %q not in %v", c.Format, Formats)
	}
	if c.Path == "" {
		v.fail("coverage.path is empty")
	}
	if len(c.ProducedBy) == 0 {
		v.fail("coverage.produced_by is empty")
	}
	for _, n := range c.ProducedBy {
		if _, ok := v.p.CheckByName(n); !ok {
			v.fail("coverage.produced_by: unknown check %q", n)
		}
	}
	for i, pr := range c.Produce {
		if len(pr.Argv) == 0 {
			v.fail("coverage.produce[%d].argv is empty", i)
		}
	}
	v.globs("coverage.exclude", c.Exclude, false)
}

func (v *validator) cacheEnv(m map[string]string) {
	for k, val := range m {
		if val != "cache" && val != "tmp" && !strings.ContainsAny(val, "{") {
			v.fail("cache_env.%s = %q: want cache, tmp or a {run}/{cache}/{tmp} template", k, val)
		}
	}
}

func (v *validator) arch(a Arch) {
	switch a.Parser {
	case ParserGo, ParserNone:
	case ParserRegex:
		if a.ImportRegex == "" || a.Resolver == "" {
			v.fail("arch: parser regex needs import_regex and resolver")
		}
		v.regex("arch.import_regex", a.ImportRegex)
	default:
		v.fail("arch.parser %q not in [go regex none]", a.Parser)
	}
	if a.NativeCheck != "" {
		if c, ok := v.p.CheckByName(a.NativeCheck); !ok || c.Kind != "arch" {
			v.fail("arch.native_check %q is not an arch-kind check", a.NativeCheck)
		}
	}
}

// placeholders checks that every {name} in an argv is a builtin or declared.
func (v *validator) placeholders() {
	known := map[string]bool{}
	for _, b := range Builtins {
		known[b] = true
	}
	for n, ph := range v.p.Placeholders {
		known[n] = true
		set := 0
		for _, ok := range []bool{len(ph.Argv) > 0, ph.File != "", len(ph.Glob) > 0} {
			if ok {
				set++
			}
		}
		if set != 1 {
			v.fail("placeholders.%s needs exactly one of argv, file, glob", n)
		}
		if ph.File != "" {
			if ph.Regex == "" {
				v.fail("placeholders.%s: file needs regex", n)
			}
			v.regex("placeholders."+n+".regex", ph.Regex)
		}
	}
	check := func(where string, argv []string) {
		for _, a := range argv {
			for _, m := range placeholderRe.FindAllStringSubmatch(a, -1) {
				if !known[m[1]] {
					v.fail("%s: unknown placeholder {%s}", where, m[1])
				}
			}
		}
	}
	all := append([]Check{}, v.p.Checks...)
	for _, vr := range v.p.Variants {
		all = append(all, vr.Checks...)
	}
	for _, c := range all {
		check("checks."+c.Name, c.Argv)
		check("checks."+c.Name+".coverage_args", c.CoverageArgs)
		check("checks."+c.Name+".fallback_argv", c.FallbackArgv)
	}
	for i, pr := range v.p.Coverage.Produce {
		check(fmt.Sprintf("coverage.produce[%d]", i), pr.Argv)
	}
	if v.p.PostEdit != nil {
		check("post_edit.format_file", v.p.PostEdit.FormatFile)
		check("post_edit.lint_dir", v.p.PostEdit.LintDir)
	}
}

func (v *validator) variants() {
	seen := map[string]bool{}
	for _, vr := range v.p.Variants {
		if vr.Name == "" || seen[vr.Name] {
			v.fail("variants: empty or duplicate name %q", vr.Name)
		}
		seen[vr.Name] = true
		v.markers("variants."+vr.Name+".detect", vr.Detect, true)
		v.checks("variants."+vr.Name+".checks", vr.Checks)
		if vr.Coverage != nil && vr.Coverage.Format != "" && !slices.Contains(Formats, vr.Coverage.Format) {
			v.fail("variants.%s.coverage.format %q not in %v", vr.Name, vr.Coverage.Format, Formats)
		}
	}
}

func trimDirSlash(gs []string) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		out = append(out, strings.TrimSuffix(g, "/"))
	}
	return out
}
