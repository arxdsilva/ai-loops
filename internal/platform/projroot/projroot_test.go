package projroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPrefersPlatformProjectEnv(t *testing.T) {
	for _, k := range projectEnv {
		t.Setenv(k, "")
	}
	for _, k := range projectEnv {
		t.Run(k, func(t *testing.T) {
			for _, o := range projectEnv {
				t.Setenv(o, "")
			}
			t.Setenv(k, "/proj/"+k)
			if got := Find(t.TempDir()); got != "/proj/"+k {
				t.Fatalf("Find = %q, want /proj/%s", got, k)
			}
		})
	}
}

func TestFindWalksUpToMarker(t *testing.T) {
	for _, k := range projectEnv {
		t.Setenv(k, "")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, _ := filepath.EvalSymlinks(Find(sub))
	want, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Fatalf("Find = %q, want %q", got, want)
	}
}
