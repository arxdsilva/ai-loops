package guard_test

import (
	"strings"
	"testing"

	"github.com/arxdsilva/ai-loops/internal/application/guard"
)

func TestCheckRace(t *testing.T) {
	deny, ok := true, false
	cases := []struct {
		name, cmd string
		deny      bool
	}{
		{"plain", "go test ./...", deny},
		{"count only", "go test -count=1 ./...", deny},
		{"race", "go test -race ./...", ok},
		{"race true", "go test -race=true ./...", ok},
		{"race after pkgs", "go test ./... -race -count=1", ok},
		{"race false", "go test -race=false ./...", deny},
		{"race on then off", "go test -race -race=false ./...", deny},
		{"race off then on", "go test -race=false -race ./...", ok},
		{"race only after -args", "go test ./... -args -race", deny},
		{"go1.22", "go1.22 test ./...", deny},
		{"go1.22 race", "go1.22.1 test -race ./...", ok},
		{"abs path go", "/usr/local/go/bin/go test ./...", deny},
		{"go -C", "go -C sub test ./...", deny},
		{"GOFLAGS inline", "GOFLAGS=-race go test ./...", ok},
		{"GOFLAGS quoted", `GOFLAGS="-p=2 -race" go test ./...`, ok},
		{"GOFLAGS no race", "GOFLAGS=-p=2 go test ./...", deny},
		{"GOFLAGS race=false", "GOFLAGS=-race=false go test ./...", deny},
		{"export earlier", "export GOFLAGS=-race && go test ./...", ok},
		{"assign earlier", "GOFLAGS=-race; go test ./...", ok},
		{"inline does not persist", "GOFLAGS=-race go build ./... && go test ./...", deny},
		{"env prefix", "env GOFLAGS=-race go test ./...", ok},
		{"env no race", "env FOO=1 go test ./...", deny},
		{"env -u", "env -u FOO go test ./...", deny},
		{"xargs", "echo ./... | xargs go test", deny},
		{"xargs race", "echo ./... | xargs -n 1 go test -race", ok},
		{"after cd", "cd sub && go test ./...", deny},
		{"second command", "go build ./... && go test ./...", deny},
		{"time prefix", "time go test ./...", deny},
		{"sh -c", `sh -c 'go test ./...'`, deny},
		{"bash -c race", `bash -c "go test -race ./..."`, ok},
		{"bash -lc", `bash -lc "go test ./..."`, deny},
		{"nested sh", `sh -c "bash -c 'go test ./...'"`, deny},
		{"sh -c with GOFLAGS", `GOFLAGS=-race sh -c 'go test ./...'`, ok},
		{"go test -list", "go test -list . ./...", ok},
		{"go test -list=", "go test -list=Test ./...", ok},
		{"go help test", "go help test", ok},
		{"go vet", "go vet ./...", ok},
		{"go build", "go build ./...", ok},
		{"go tool", "go tool cover -html=c.out", ok},
		{"quoted mention", `echo "go test ./..."`, ok},
		{"comment", "ls # go test ./...", ok},
		{"heredoc body", "cat <<EOF\ngo test ./...\nEOF", ok},
		{"gotestsum plain", "gotestsum ./...", deny},
		{"gotestsum race", "gotestsum --format pkgname -- -race ./...", ok},
		{"gotestsum no race", "gotestsum --format pkgname -- -count=1 ./...", deny},
		{"gotestsum version", "gotestsum --version", ok},
		{"richgo", "richgo test ./...", deny},
		{"richgo race", "richgo test -race ./...", ok},
		{"unterminated quote", `go test "./...`, ok},
		{"empty", "", ok},
		{"timeout", "timeout 60 go test ./...", deny},
		{"timeout opts", "timeout -s KILL -k 5 60 go test ./...", deny},
		{"timeout race", "timeout 60 go test -race ./...", ok},
		{"stdbuf", "stdbuf -oL go test ./...", deny},
		{"stdbuf -o L", "stdbuf -o L go test ./...", deny},
		{"nice -n", "nice -n 10 go test ./...", deny},
		{"nohup", "nohup go test ./...", deny},
		{"watch", `watch -n 5 "go test ./..."`, deny},
		{"watch words", "watch -n 5 go test ./...", deny},
		{"watch race", `watch -n 5 "go test -race ./..."`, ok},
		{"command", "command go test ./...", deny},
		{"exec", "exec go test ./...", deny},
		{"caffeinate", "caffeinate -i go test ./...", deny},
		{"ionice", "ionice -c 3 go test ./...", deny},
		{"chrt", "chrt 10 go test ./...", deny},
		{"setsid", "setsid go test ./...", deny},
		{"unbuffer", "unbuffer go test ./...", deny},
		{"nested wrappers", "nohup nice -n 5 timeout 60 stdbuf -oL go test ./...", deny},
		{"nested wrappers race", "nohup nice -n 5 timeout 60 stdbuf -oL go test -race ./...", ok},
		{"eval quoted", `eval "go test ./..."`, deny},
		{"eval words", "eval go test ./...", deny},
		{"eval race", `eval "go test -race ./..."`, ok},
		{"backticks", "echo `go test ./...`", deny},
		{"backticks race", "echo `go test -race ./...`", ok},
		{"dollar paren", "x=$(go test ./...)", deny},
		{"heredoc sh", "sh <<EOF\ngo test ./...\nEOF", deny},
		{"heredoc bash -s", "bash -s <<'X'\ngo test ./...\nX", deny},
		{"heredoc sh race", "sh <<EOF\ngo test -race ./...\nEOF", ok},
		{"heredoc cat still ok", "cat <<EOF\ngo test ./...\nEOF", ok},
	}
	for _, tc := range cases {
		got := guard.CheckRace(tc.cmd)
		if tc.deny && !strings.HasPrefix(got, "race_guard:") {
			t.Errorf("%s: %q want deny, got %q", tc.name, tc.cmd, got)
		}
		if !tc.deny && got != "" {
			t.Errorf("%s: %q want allow, got %q", tc.name, tc.cmd, got)
		}
	}
}
