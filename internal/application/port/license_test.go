package port

import (
	"testing"
	"testing/fstest"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

// Every generated package must carry the license, because marketplaces
// install a package directory without the repository root.
func TestPackagesCarryLicense(t *testing.T) {
	p, src := loadFixture(t)
	src["LICENSE"] = &fstest.MapFile{Data: []byte("MIT License\n")}
	for _, target := range []plugin.Platform{plugin.Codex, plugin.Cursor, plugin.Gemini, plugin.OpenCode, plugin.Amp, plugin.Windsurf, plugin.Cline} {
		files, _, err := Build(p, src, target, Options{Version: "v9.9.9"})
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		found := false
		for _, f := range files {
			if f.Path == "LICENSE" && string(f.Data) == "MIT License\n" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s package has no LICENSE", target)
		}
	}
}
