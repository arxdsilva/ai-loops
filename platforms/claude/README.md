# loops for Claude Code

The repository root is the Claude Code plugin (`.claude-plugin/`, `agents/`, `skills/`, `hooks/hooks.json`); nothing is generated for Claude. See specs/PLATFORMS.md §3.1.

## Install

    /plugin marketplace add arxdsilva/ai-loops
    /plugin install loops@ai-loops

Commands: `/loops:work`, `/loops:init`, `/loops:status`, `/loops:report`.

The root `bin/loops` shim builds `cmd/loops` from the checkout on first use (Go required).
