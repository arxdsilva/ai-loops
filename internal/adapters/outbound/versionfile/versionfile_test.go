package versionfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `{
  "name": "loops",
  "version": "0.3.0",
  "description": "has \"version\": \"9.9.9\" inside",
  "author": {
    "name": "A"
  }
}
`

func tmp(t *testing.T, data string) (*versionFile, string) {
	p := filepath.Join(t.TempDir(), "plugin.json")
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(p), p
}

func TestReadWrite(t *testing.T) {
	f, p := tmp(t, sample)
	if v, err := f.ReadVersion(); err != nil || v != "0.3.0" {
		t.Fatalf("%q %v", v, err)
	}
	if err := f.WriteVersion("0.4.0"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := strings.Replace(sample, `"version": "0.3.0"`, `"version": "0.4.0"`, 1)
	if string(got) != want {
		t.Fatalf("diff:\n%s", got)
	}
	if v, _ := f.ReadVersion(); v != "0.4.0" {
		t.Fatal(v)
	}
	var m map[string]any
	if json.Unmarshal(got, &m) != nil {
		t.Fatal("invalid json")
	}
}

func TestErrors(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "nope")).ReadVersion(); err == nil {
		t.Fatal("missing file")
	}
	if err := New(filepath.Join(t.TempDir(), "nope")).WriteVersion("1.0.0"); err == nil {
		t.Fatal("missing file write")
	}
	f, _ := tmp(t, `{"name":"x"}`)
	if _, err := f.ReadVersion(); err == nil {
		t.Fatal("no version")
	}
	if err := f.WriteVersion("1.0.0"); err == nil {
		t.Fatal("no version write")
	}
	f, _ = tmp(t, `{bad`)
	if _, err := f.ReadVersion(); err == nil {
		t.Fatal("bad json")
	}
	f, _ = tmp(t, sample)
	if err := f.WriteVersion(`1"0`); err == nil {
		t.Fatal("bad version accepted")
	}
}
