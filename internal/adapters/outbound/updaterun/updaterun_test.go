package updaterun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arxdsilva/ai-loops/internal/application/updatecheck"
)

const helperEnv = "UPDATERUN_HELPER_MODE"

// TestHelperProcess is the child: it is re-executed by the tests below and
// does nothing in a normal run.
func TestHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		return
	}
	switch mode {
	case "ok":
		fmt.Fprint(os.Stdout, "out-line\n")
		fmt.Fprint(os.Stderr, "err-line\n")
		os.Exit(0)
	case "exit3":
		fmt.Fprint(os.Stdout, "before failing\n")
		os.Exit(3)
	case "block":
		time.Sleep(time.Hour)
	case "stdin":
		b, _ := io.ReadAll(os.Stdin)
		fmt.Fprintf(os.Stdout, "stdin=%d", len(b))
		os.Exit(0)
	case "big":
		chunk := strings.Repeat("x", 1023) + "\n"
		for i := 0; i < 200; i++ {
			fmt.Fprint(os.Stdout, chunk)
		}
		fmt.Fprint(os.Stdout, "END")
		os.Exit(0)
	}
	os.Exit(9)
}

func helperArgv(mode string) []string {
	return []string{os.Args[0], "-test.run=^TestHelperProcess$"}
}

func run(t *testing.T, ctx context.Context, mode string) (string, error) {
	t.Helper()
	t.Setenv(helperEnv, mode)
	return New().Run(ctx, helperArgv(mode))
}

func TestRunSuccessReturnsBothStreams(t *testing.T) {
	out, err := run(t, context.Background(), "ok")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "out-line") || !strings.Contains(out, "err-line") {
		t.Fatalf("out = %q", out)
	}
}

func TestRunNonZeroExitKeepsOutput(t *testing.T) {
	out, err := run(t, context.Background(), "exit3")
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(out, "before failing") {
		t.Fatalf("out = %q", out)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	_, err := New().Run(context.Background(), []string{"loops-no-such-command-xyz"})
	if !errors.Is(err, updatecheck.ErrNoCommand) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunInjectedLookup(t *testing.T) {
	r := &runner{lookPath: func(string) (string, error) { return "", errors.New("nope") }}
	_, err := r.Run(context.Background(), []string{"claude"})
	if !errors.Is(err, updatecheck.ErrNoCommand) || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunEmptyArgv(t *testing.T) {
	for _, argv := range [][]string{nil, {}} {
		if _, err := New().Run(context.Background(), argv); err == nil {
			t.Fatalf("argv %v: want error", argv)
		}
	}
}

func TestRunDeadlineKillsChildWithinBound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := run(t, ctx, "block")
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if bound := 200*time.Millisecond + New().waitDelay + 3*time.Second; elapsed > bound {
		t.Fatalf("took %v, bound %v", elapsed, bound)
	}
}

func TestRunStdinIsEOF(t *testing.T) {
	out, err := run(t, context.Background(), "stdin")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "stdin=0") {
		t.Fatalf("out = %q", out)
	}
}

func TestRunKeepsLastBytesOfLargeOutput(t *testing.T) {
	out, err := run(t, context.Background(), "big")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(out) > New().maxOutput {
		t.Fatalf("len = %d, cap %d", len(out), New().maxOutput)
	}
	if !strings.HasSuffix(out, "END") {
		t.Fatalf("tail = %q", out[len(out)-10:])
	}
}

func TestTailBufferSmallAndExact(t *testing.T) {
	b := &tailBuffer{max: 4}
	n, _ := b.Write([]byte("ab"))
	b.Write([]byte("cdef"))
	if n != 2 || b.String() != "cdef" {
		t.Fatalf("n=%d s=%q", n, b.String())
	}
	b.Write([]byte("0123456789"))
	if b.String() != "6789" {
		t.Fatalf("s=%q", b.String())
	}
}
