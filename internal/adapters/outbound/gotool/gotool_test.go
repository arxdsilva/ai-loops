package gotool

import (
	"context"
	"strings"
	"testing"
)

func TestRunAndHas(t *testing.T) {
	r := New()
	out, err := r.Run(context.Background(), t.TempDir(), "sh", "-c", "echo hi; echo err >&2; exit 3")
	if err == nil || !strings.Contains(out, "hi") || !strings.Contains(out, "err") {
		t.Fatalf("%q %v", out, err)
	}
	if !r.Has("sh") || r.Has("definitely-not-a-binary-xyz") {
		t.Fatal("Has")
	}
	if _, err := r.Run(context.Background(), "", "definitely-not-a-binary-xyz"); err == nil {
		t.Fatal("want err")
	}
}
