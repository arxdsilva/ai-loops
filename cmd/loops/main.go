// Command loops is the tooling behind the loops Claude Code plugin.
package main

import (
	"context"
	"os"

	"github.com/arxdsilva/ai-loops/internal/compose/loops"
)

func main() {
	os.Exit(loops.Main(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
