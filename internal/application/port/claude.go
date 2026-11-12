package port

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// claudeVerbatim are copied into the directory package unchanged when the
// source has them: the manifest, its icon, the hooks, the status line
// settings and the live pane (hooks.json loads ../mod/register.js).
var claudeVerbatim = []string{
	".claude-plugin/plugin.json",
	".claude-plugin/icon.png",
	"hooks/hooks.json",
	"settings.json",
	"mod/register.js",
	"mod/view.js",
}

// buildClaude emits platforms/claude/ (specs/PLATFORMS.md §3.1): the
// plugin-directory package. The repository root stays the marketplace
// plugin; this copy is the same plugin with a shim that downloads nothing
// (claudeShim), because the Claude plugin directory refuses a plugin that
// downloads a file and runs it.
func buildClaude(p plugin.Plugin, src fs.FS, o Options) ([]plugin.File, []string, error) {
	files, err := baseFiles(p, src, plugin.Claude, o)
	if err != nil {
		return nil, nil, err
	}
	files = putFile(files, claudeShim(o))
	for _, a := range p.Agents {
		data, err := fs.ReadFile(src, "agents/"+a.Name+".md")
		if err != nil {
			return nil, nil, fmt.Errorf("port: %w", err)
		}
		files = putFile(files, plugin.File{Path: "agents/" + a.Name + ".md", Data: data})
	}
	for _, name := range claudeVerbatim {
		data, err := fs.ReadFile(src, name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("port: %w", err)
		}
		files = putFile(files, plugin.File{Path: name, Data: data})
	}
	readme, err := claudeReadme(p, src, o)
	if err != nil {
		return nil, nil, err
	}
	return putFile(files, plugin.File{Path: "README.md", Data: []byte(readme)}), nil, nil
}

// claudeDisclosure is the part of the repository README's "What it runs on
// your machine" section that holds for the directory package too: from the
// hooks bullet up to (not including) the network bullet.
const (
	claudeDisclosureHeading = "## What it runs on your machine"
	claudeDisclosureStart   = "- **Hooks**"
	claudeDisclosureEnd     = "- **Network**"
	repoBlob                = "https://github.com/arxdsilva/ai-loops/blob/main/"
)

// claudeReadme is the directory package's README: install, then what the
// plugin runs, sends and changes (the directory asks for both).
func claudeReadme(p plugin.Plugin, src fs.FS, o Options) (string, error) {
	disclosure := ""
	if data, err := fs.ReadFile(src, "README.md"); err == nil {
		s := string(data)
		h := strings.Index(s, claudeDisclosureHeading)
		if h < 0 {
			return "", fmt.Errorf("port: README.md has no %q section (the Claude directory package reuses it)", claudeDisclosureHeading)
		}
		s = s[h:]
		i, j := strings.Index(s, claudeDisclosureStart), strings.Index(s, claudeDisclosureEnd)
		if i < 0 || j < i {
			return "", fmt.Errorf("port: README.md %q must hold a %q bullet before a %q bullet", claudeDisclosureHeading, claudeDisclosureStart, claudeDisclosureEnd)
		}
		disclosure = strings.ReplaceAll(s[i:j], "](docs/", "]("+repoBlob+"docs/")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("port: %w", err)
	}
	return fmt.Sprintf(`# %[1]s for Claude Code

%[2]s

This is the Claude plugin directory package of [github.com/arxdsilva/ai-loops](https://github.com/arxdsilva/ai-loops). It is the same plugin as the marketplace install, except that it downloads nothing: it runs a `+"`loops`"+` binary you install once.

## Install

1. Install %[1]s from the Claude Code plugin directory.
2. Install the `+"`loops`"+` binary once, in a terminal: `+"`npx ai-loops bin`"+` (Node 18+). It downloads the release binary for your OS from GitHub, checks it against the release's `+"`checksums.txt`"+` and puts it in `+"`~/.local/bin`"+`. Without Node: `+"`go install github.com/arxdsilva/ai-loops/cmd/loops@%[3]s`"+`, or the other ways in the [install guide](%[4]sdocs/GUIDE.md#install).
3. In your repository: `+"`/%[1]s:init`"+`, then `+"`/%[1]s:work <ticket>`"+`.

Prefer automatic setup? `+"`/plugin marketplace add arxdsilva/ai-loops`"+` and `+"`/plugin install %[1]s@ai-loops`"+` install the same plugin with a shim that downloads the binary on first use.

## What it runs on your machine

Everything goes through `+"`bin/loops`"+`, a shell script that runs the `+"`loops`"+` binary on your PATH (or in `+"`~/.local/bin`"+` or the loops cache), preferring the one whose version matches this package (%[3]s). It downloads nothing. Without a binary every hook does nothing, and session start tells Claude to ask you to install it.

%[5]s- **Network**: none from this package. During a run, your agents use your ticket and PR tools (`+"`gh`"+`, `+"`glab`"+`, connectors) and web research; the `+"`loops`"+` binary checks proxy.golang.org for a newer version at most once a day (turn it off with `+"`\"update_check\": false`"+` in `+"`.loops/config.json`"+` or `+"`LOOPS_NO_UPDATE_CHECK=1`"+`). Run records stay in `+"`.loops/runs/`"+` in your repository.

MIT license.
`, p.Name, p.Description, o.Version, repoBlob, disclosure), nil
}

// claudeShim is bin/loops for the directory package: it finds an installed
// loops (PATH, ~/.local/bin, then the cache), preferring this version, and
// never downloads or builds. Without one, hooks and the status line exit 0
// with no effect (hooks fail open), and session-start adds a note asking
// the user to install it.
func claudeShim(o Options) plugin.File {
	ver := strings.TrimPrefix(o.Version, "v")
	script := `#!/bin/sh
# loops shim for the Claude plugin directory package (generated by loops port
# build - do not edit; specs/PLATFORMS.md §3.1). It downloads nothing: it runs
# the loops binary installed on this machine (npx ai-loops bin), preferring
# the one that matches this package.
set -e
root="${CLAUDE_PLUGIN_ROOT:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}"
export LOOPS_PLUGIN_ROOT="$root"
V='` + ver + `'
cache="${CLAUDE_PLUGIN_DATA:-${XDG_CACHE_HOME:-$HOME/.cache}/loops}"
any=
oldifs=$IFS
IFS=:
for d in $PATH "$HOME/.local/bin"; do
  for c in "${d:-.}/loops" "${d:-.}/loops.exe"; do
    if [ -f "$c" ] && [ -x "$c" ] && [ "$(dd if="$c" bs=2 count=1 2>/dev/null)" != '#!' ]; then
      if [ "$("$c" version 2>/dev/null || true)" = "loops $V" ]; then
        IFS=$oldifs
        exec "$c" "$@"
      fi
      [ -n "$any" ] || any=$c
    fi
  done
done
IFS=$oldifs
for c in "$cache/loops-$V" "$cache/loops-$V.exe"; do
  if [ -x "$c" ]; then
    exec "$c" "$@"
  fi
done
if [ -n "$any" ]; then
  exec "$any" "$@"
fi
msg='the loops binary is not installed. Run this once in a terminal: npx ai-loops bin (Node 18+), or see https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#install'
case "$1 $2" in
  'hook session-start') echo "loops: $msg. Ask the user to install it before running a loops command." ;;
  hook* | 'status --subagent') ;;
  *)
    echo "loops: $msg" >&2
    exit 1
    ;;
esac
exit 0
`
	return plugin.File{Path: "bin/loops", Data: []byte(script), Exec: true}
}
