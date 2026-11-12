// Package ailoops embeds the canonical sources of the loops plugin (agents,
// skills, specs, templates, Claude hooks config, ROLES.md) so the `loops`
// binary carries the whole plugin: `loops port build` generates the
// per-platform packages and `loops install` places them, even when the
// binary came from `go install` without a checkout.
package ailoops

import "embed"

// Source is the canonical plugin tree, rooted at the repository root.
//
//go:embed agents skills specs templates hooks/hooks.json ROLES.md LICENSE .claude-plugin/plugin.json
//go:embed .claude-plugin/icon.png settings.json mod/register.js mod/view.js README.md
var Source embed.FS
