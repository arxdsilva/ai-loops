package versionfile

import "path/filepath"

// PluginVersions reads an installed plugin's version from its root.
type PluginVersions struct{}

// PluginVersion returns the "version" of <root>/.claude-plugin/plugin.json.
func (PluginVersions) PluginVersion(root string) (string, error) {
	return New(filepath.Join(root, ".claude-plugin", "plugin.json")).ReadVersion()
}
