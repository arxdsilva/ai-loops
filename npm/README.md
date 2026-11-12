# ai-loops

Install **loops**, the multi-agent coding workflow that proves its work, with one command. Give your AI coding agent a ticket and get back a pull request with proof that every acceptance criteria works.

```sh
npx ai-loops                        # Claude Code (uses the claude CLI)
npx ai-loops --target copilot       # or codex, cursor, gemini, opencode, amp, windsurf, cline
```

Then, in your repository: `/loops:init` and `/loops:work #42`.

- `--scope user|project|local` picks where Claude Code installs the plugin (default `user`). Installer platforms take `project` (default) or `user`.
- Platforms that need the `loops` binary get it from the GitHub release that matches this package's version, checked against its `checksums.txt`, in `~/.local/bin` (change with `--bin-dir`).
- `npx ai-loops bin` installs only the `loops` binary (the Claude plugin directory package needs it, since that package downloads nothing).
- Any other arguments run that `loops` command: `npx ai-loops version`, `npx ai-loops report`.

Everything else, including what each platform supports: [github.com/arxdsilva/ai-loops](https://github.com/arxdsilva/ai-loops) and [the install guide](https://github.com/arxdsilva/ai-loops/blob/main/docs/GUIDE.md#install). MIT license, zero dependencies.
