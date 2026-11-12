// Package port turns the canonical plugin sources into per-platform
// packages (specs/PLATFORMS.md). Load parses the canonical tree from an
// fs.FS; Build runs one platform's emitter. Both are pure: no os, no I/O
// beyond the given fs.FS.
package port

import (
	"fmt"
	"io/fs"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// Commands are the skills that are also user slash commands.
var Commands = []string{"work", "init", "status", "report"}

// Options tunes a build.
type Options struct {
	// Models maps a tier to a platform model id; a missing or empty entry
	// means "inherit the session model" (omit the field).
	Models map[plugin.Tier]string
	// Module is the Go module path the package shims `go install`
	// (github.com/arxdsilva/ai-loops); Version the tag (v0.2.0).
	Module  string
	Version string
}

// Load parses the canonical sources (embed.go's layout) into a Plugin:
// .claude-plugin/plugin.json metadata, skills/*/SKILL.md frontmatter,
// agents/*.md frontmatter + body, hooks/hooks.json bindings. Worker P1.
func Load(src fs.FS) (plugin.Plugin, error) {
	p, err := loadPlugin(src)
	if err != nil {
		return plugin.Plugin{}, err
	}
	if p.Skills, err = loadSkills(src); err != nil {
		return plugin.Plugin{}, err
	}
	if p.Agents, err = loadAgents(src); err != nil {
		return plugin.Plugin{}, err
	}
	if p.Hooks, err = loadHooks(src); err != nil {
		return plugin.Plugin{}, err
	}
	return p, nil
}

// Build returns the files of target's package (paths relative to
// platforms/<target>/) and human-readable warnings for anything lossy
// (e.g. a tool allowlist the platform cannot express). Worker P1/P2.
func Build(p plugin.Plugin, src fs.FS, target plugin.Platform, o Options) ([]plugin.File, []string, error) {
	emit, ok := emitters[target]
	if !ok {
		return nil, nil, fmt.Errorf("port: unknown platform %q", target)
	}
	return emit(p, src, withDefaults(p, o))
}

// emitter builds one platform's package.
type emitter func(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error)

// emitters maps platforms to their builders (one file per platform).
var emitters = map[plugin.Platform]emitter{
	plugin.Claude:   buildClaude,   // claude.go   (P1): the plugin-directory package (no-download shim)
	plugin.Copilot:  buildCopilot,  // copilot.go  (P1): README only — Copilot reads the root
	plugin.Codex:    buildCodex,    // codex.go    (P1)
	plugin.Cursor:   buildCursor,   // cursor.go   (P1)
	plugin.Gemini:   buildGemini,   // gemini.go   (P2)
	plugin.OpenCode: buildOpenCode, // opencode.go (P2)
	plugin.Amp:      buildAmp,      // amp.go      (P2)
	plugin.Windsurf: buildWindsurf, // windsurf.go (P2)
	plugin.Cline:    buildCline,    // cline.go    (P2)
}

// RootFiles are files the generator writes at the REPOSITORY root (paths
// relative to it): .agents/plugins/marketplace.json (Codex) and
// .cursor-plugin/marketplace.json (Cursor), pointing at platforms/<name>.
// Worker P1.
func RootFiles(p plugin.Plugin) []plugin.File {
	return []plugin.File{codexMarketplace(p), cursorMarketplace(p)}
}
