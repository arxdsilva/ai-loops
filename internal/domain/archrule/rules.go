package archrule

import "strings"

// Rules is the per-language data the rules read (a language pack's "arch"
// block, research/languages/TOOLING.md §6.2). Evaluate uses GoRules;
// EvaluateWith takes the fields as given, so an empty list means "none".
type Rules struct {
	// ForbiddenInDomain lists the non-internal imports the domain must not
	// use. An entry matches the import equal to it or below it (the next
	// character is one of . / \ :); a trailing "*" or separator makes it a
	// plain prefix.
	ForbiddenInDomain []string `json:"forbidden_in_domain,omitempty"`
	// ImpureCalls gates the Go purity rules ("time.Now", "math/rand",
	// "math/rand/v2", "go"). Non-Go parsers apply the pack's list while
	// scanning and record Package.Impure hits, which are always reported.
	ImpureCalls []string `json:"impure_calls,omitempty"`
	// ThirdPartyForbiddenIn names the layers ("domain", "application", "pkg",
	// ...) that may import only the standard library and the module itself.
	ThirdPartyForbiddenIn []string `json:"third_party_forbidden_in,omitempty"`
	// Limits overrides the size-warning limits by rule name: "methods",
	// "fields", "iface", "lines", "files". Zero or missing keeps the default.
	Limits map[string]int `json:"limits,omitempty"`
}

// GoRules is today's Go checker: the data of specs/languages/go/pack.json.
func GoRules() Rules {
	return Rules{
		ForbiddenInDomain:     []string{"os", "net/http"},
		ImpureCalls:           []string{"time.Now", "math/rand", "math/rand/v2", "go"},
		ThirdPartyForbiddenIn: []string{"domain", "application", "pkg"},
	}
}

func (r Rules) has(call string) bool {
	for _, c := range r.ImpureCalls {
		if c == call {
			return true
		}
	}
	return false
}

func (r Rules) forbidden(imp string) bool {
	for _, e := range r.ForbiddenInDomain {
		if matchImport(e, imp) {
			return true
		}
	}
	return false
}

func (r Rules) noThirdParty(l layer) bool {
	for _, n := range r.ThirdPartyForbiddenIn {
		if n == l.String() {
			return true
		}
	}
	return false
}

func (r Rules) limit(rule string, def int) int {
	if n := r.Limits[rule]; n > 0 {
		return n
	}
	return def
}

// matchImport reports whether imp is entry or lies below it.
func matchImport(entry, imp string) bool {
	entry = strings.TrimSuffix(entry, "*")
	if entry == "" {
		return false
	}
	if imp == entry {
		return true
	}
	if !strings.HasPrefix(imp, entry) {
		return false
	}
	if isImportSep(entry[len(entry)-1]) {
		return true
	}
	return isImportSep(imp[len(entry)])
}

func isImportSep(c byte) bool { return c == '.' || c == '/' || c == '\\' || c == ':' }

// ConventionalLayers is the default layout for a non-Go pack when the
// project config sets no layers: a layer is a directory named after it
// anywhere in the path ("**/domain" covers src/main/java/com/x/domain,
// src/domain, crates/domain/src, Sources/Domain via the capitalised form).
// The deepest match wins, as with prefixes. The project-reference resolver
// (C#) adds the dotted project names of the Clean Architecture template
// (MyApp.Domain, MyApp.Application, MyApp.Infrastructure).
func ConventionalLayers(resolver string) Layers {
	names := func(ns ...string) []string {
		var out []string
		for _, n := range ns {
			segs := strings.Split(n, "/")
			for i, s := range segs {
				segs[i] = strings.ToUpper(s[:1]) + s[1:]
			}
			out = append(out, "**/"+n, "**/"+strings.Join(segs, "/"))
		}
		return out
	}
	l := Layers{
		Domain:      names("domain"),
		Application: names("application", "usecase", "usecases"),
		Ports:       names("ports"),
		Inbound:     names("adapters/inbound", "adapters/in"),
		Outbound:    names("adapters/outbound", "adapters/out"),
		Adapters:    names("adapters", "infrastructure", "infra"),
		Platform:    names("platform"),
		Compose:     names("compose"),
		Cmd:         names("cmd"),
		Pkg:         names("pkg"),
	}
	if resolver == "project-reference" {
		l.Domain = append(l.Domain, "**/*.Domain")
		l.Application = append(l.Application, "**/*.Application")
		l.Adapters = append(l.Adapters, "**/*.Infrastructure", "**/*.Adapters")
	}
	return l
}
