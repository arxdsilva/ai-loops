package versionfile

import (
	"os"
	"path/filepath"
	"testing"
)

// pluginRoot returns a temp plugin root; when data is non-empty it holds
// .claude-plugin/plugin.json with that content.
func pluginRoot(t *testing.T, data string) string {
	t.Helper()
	root := t.TempDir()
	if data == "" {
		return root
	}
	dir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestPluginVersion(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{name: "reads version", data: sample, want: "0.3.0"},
		{name: "missing plugin.json", data: "", wantErr: true},
		{name: "invalid JSON", data: "{not json", wantErr: true},
		{name: "no version key", data: `{"name": "loops"}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PluginVersions{}.PluginVersion(pluginRoot(t, tc.data))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
