package portfs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/domain/plugin"
)

func fl(p, d string, exec bool) plugin.File { return plugin.File{Path: p, Data: []byte(d), Exec: exec} }

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteImageExecAndPrune(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pkg")
	files := []plugin.File{fl("a.txt", "A", false), fl("bin/loops", "#!/bin/sh\n", true), fl("d/e/f.md", "F", false)}
	fsys := New()
	if err := fsys.Write(dir, files); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "bin/loops")); fi.Mode()&0o111 == 0 {
		t.Error("exec bit lost")
	}
	if fi, _ := os.Stat(filepath.Join(dir, "a.txt")); fi.Mode()&0o111 != 0 {
		t.Error("unexpected exec bit")
	}
	if _, err := os.Stat(filepath.Join(dir, Marker)); err != nil {
		t.Error("marker missing")
	}
	os.WriteFile(filepath.Join(dir, "stale.txt"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(dir, "old/sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "old/sub/z"), []byte("x"), 0o644)
	if err := fsys.Write(dir, files); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"stale.txt", "old"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s not removed", gone)
		}
	}
	if d, err := fsys.Diff(dir, files); err != nil || len(d) != 0 {
		t.Errorf("diff after write: %v %v", d, err)
	}
}

func TestWriteRefusesForeignDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mine.txt"), []byte("keep"), 0o644)
	if err := New().Write(dir, []plugin.File{fl("a", "A", false)}); err == nil {
		t.Fatal("want refusal")
	}
	if read(t, filepath.Join(dir, "mine.txt")) != "keep" {
		t.Error("foreign file touched")
	}
	// empty existing dir is fine
	if err := New().Write(t.TempDir(), []plugin.File{fl("a", "A", false)}); err != nil {
		t.Error(err)
	}
}

func TestWriteRejectsUnsafePaths(t *testing.T) {
	for _, p := range []string{"../x", "/abs", "a/../../x", ""} {
		if err := New().Write(filepath.Join(t.TempDir(), "p"), []plugin.File{fl(p, "x", false)}); err == nil {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestWriteDoesNotFollowSymlinks(t *testing.T) {
	outside := t.TempDir()
	dir := filepath.Join(t.TempDir(), "pkg")
	if err := New().Write(dir, []plugin.File{fl("seed", "s", false)}); err != nil {
		t.Fatal(err)
	}
	os.Symlink(outside, filepath.Join(dir, "linked"))
	os.Symlink(filepath.Join(outside, "target"), filepath.Join(dir, "file"))
	if err := New().Write(dir, []plugin.File{fl("linked/x.txt", "X", false), fl("file", "F", false)}); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("wrote outside dir: %v", entries)
	}
	if read(t, filepath.Join(dir, "linked/x.txt")) != "X" || read(t, filepath.Join(dir, "file")) != "F" {
		t.Error("content wrong")
	}
}

func TestDiff(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pkg")
	files := []plugin.File{fl("a", "A", false), fl("b", "B", false), fl("c", "C", false), fl("x/sh", "S", true)}
	fsys := New()
	if err := fsys.Write(dir, files); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "a"), []byte("changed"), 0o644)
	os.Remove(filepath.Join(dir, "b"))
	os.WriteFile(filepath.Join(dir, "extra"), []byte("e"), 0o644)
	os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("j"), 0o644)
	os.Chmod(filepath.Join(dir, "x/sh"), 0o644)
	got, err := fsys.Diff(dir, files)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b", "extra", "x/sh"}; !reflect.DeepEqual(got, want) {
		t.Errorf("diff = %v, want %v", got, want)
	}
	// missing dir: everything is missing
	got, err = fsys.Diff(filepath.Join(t.TempDir(), "nope"), files[:1])
	if err != nil || !reflect.DeepEqual(got, []string{Marker, "a"}) {
		t.Errorf("missing dir: %v %v", got, err)
	}
}

func TestPlace(t *testing.T) {
	root := t.TempDir()
	d1, d2 := filepath.Join(root, "x/y/one"), filepath.Join(root, "two")
	pl := []plugin.Placement{{Dest: d1, File: fl("a", "1", false)}, {Dest: d2, File: fl("b", "2", true)}}
	fsys := New()
	w, err := fsys.Place(pl, false)
	if err != nil || !reflect.DeepEqual(w, []string{d1, d2}) {
		t.Fatalf("%v %v", w, err)
	}
	if fi, _ := os.Stat(d2); fi.Mode()&0o111 == 0 {
		t.Error("exec lost")
	}
	pl[0].File.Data = []byte("new")
	if w, _ := fsys.Place(pl, false); len(w) != 0 || read(t, d1) != "1" {
		t.Errorf("existing overwritten: %v", w)
	}
	if w, _ := fsys.Place(pl, true); len(w) != 2 || read(t, d1) != "new" {
		t.Errorf("force: %v", w)
	}
	if _, err := fsys.Place([]plugin.Placement{{Dest: "rel/x", File: fl("a", "", false)}}, false); err == nil {
		t.Error("relative dest accepted")
	}
}

func TestPlaceReplacesSymlinkNotTarget(t *testing.T) {
	root, outside := t.TempDir(), filepath.Join(t.TempDir(), "victim")
	os.WriteFile(outside, []byte("orig"), 0o644)
	dest := filepath.Join(root, "link")
	os.Symlink(outside, dest)
	if w, _ := New().Place([]plugin.Placement{{Dest: dest, File: fl("a", "N", false)}}, false); len(w) != 0 {
		t.Error("skip expected")
	}
	if _, err := New().Place([]plugin.Placement{{Dest: dest, File: fl("a", "N", false)}}, true); err != nil {
		t.Fatal(err)
	}
	if read(t, outside) != "orig" || read(t, dest) != "N" {
		t.Error("followed symlink")
	}
}
