// Pure helpers for the ai-loops npm launcher (no I/O): argument parsing, the
// install plan per platform, release asset names and checksum lookup.

export const REPO = 'arxdsilva/ai-loops';
export const MARKETPLACE = 'ai-loops';
export const PLUGIN = 'loops@ai-loops';

export const PLATFORMS = ['claude', 'copilot', 'codex', 'cursor', 'gemini', 'opencode', 'amp', 'windsurf', 'cline'];
// Platforms whose files `loops install --target` places (needs the binary).
const INSTALLER = new Set(['codex', 'opencode', 'amp', 'windsurf', 'cline']);

export const USAGE = `usage:
  npx ai-loops [install] [--target <platform>] [--scope <scope>] [--force] [--bin-dir <dir>]
  npx ai-loops bin [--bin-dir <dir>]         install only the loops binary (default ~/.local/bin)
  npx ai-loops <loops command> [args...]     e.g. npx ai-loops version

install (the default) sets loops up for one agent tool; --target defaults to claude.
platforms: ${PLATFORMS.join(', ')}
scope: claude takes user, project or local (default user); installer platforms take project or user.
Any other command runs the loops binary of this package's version.
Guide: https://github.com/${REPO}/blob/main/docs/GUIDE.md#install`;

// parseArgs splits argv into {cmd:'install'|'bin'|'help'|'version'|'run', opts, rest}.
export function parseArgs(argv) {
  const args = [...argv];
  if (args[0] === '-h' || args[0] === '--help' || args[0] === 'help') return { cmd: 'help' };
  if (args[0] === '-v' || args[0] === '--version') return { cmd: 'version' };
  if (args[0] === 'bin') {
    const opts = { binDir: '' };
    for (let i = 1; i < args.length; i++) {
      const a = args[i];
      if (a === '--bin-dir' && i + 1 < args.length && !args[i + 1].startsWith('--')) opts.binDir = args[++i];
      else if (a.startsWith('--bin-dir=')) opts.binDir = a.slice(10);
      else if (a === '-h' || a === '--help') return { cmd: 'help' };
      else throw new Error(a === '--bin-dir' ? '--bin-dir needs a value' : `unknown option ${a}`);
    }
    return { cmd: 'bin', opts };
  }
  if (args.length === 0 || args[0] === 'install' || args[0].startsWith('--')) {
    if (args[0] === 'install') args.shift();
    const opts = { target: 'claude', scope: '', force: false, binDir: '' };
    while (args.length) {
      const a = args.shift();
      const val = name => {
        if (!args.length || args[0].startsWith('--')) throw new Error(`${name} needs a value`);
        return args.shift();
      };
      if (a === '--target') opts.target = val(a).toLowerCase();
      else if (a.startsWith('--target=')) opts.target = a.slice(9).toLowerCase();
      else if (a === '--scope') opts.scope = val(a);
      else if (a.startsWith('--scope=')) opts.scope = a.slice(8);
      else if (a === '--bin-dir') opts.binDir = val(a);
      else if (a === '--force') opts.force = true;
      else if (a === '-h' || a === '--help') return { cmd: 'help' };
      else throw new Error(`unknown option ${a}`);
    }
    if (!PLATFORMS.includes(opts.target)) throw new Error(`unknown platform ${opts.target} (want one of ${PLATFORMS.join(', ')})`);
    return { cmd: 'install', opts };
  }
  return { cmd: 'run', rest: args };
}

// plan lists the steps that install loops for opts.target. Each step is
// {run:[argv...], optional?:bool, fallback?:[argv...]} (a command),
// {binary:true} (download the checksummed loops binary into the bin dir),
// {gemini:true} (fetch the release source and install platforms/gemini) or
// {note:'...'} (print only).
export function plan(opts) {
  const { target, scope, force } = opts;
  switch (target) {
    case 'claude':
      return [
        { run: ['claude', 'plugin', 'marketplace', 'add', REPO], fallback: ['claude', 'plugin', 'marketplace', 'update', MARKETPLACE] },
        { run: ['claude', 'plugin', 'install', PLUGIN, '--scope', scope || 'user'] },
        { note: 'Done. In your repository run /loops:init, then /loops:work <ticket>.' },
      ];
    case 'copilot':
      return [
        { run: ['copilot', 'plugin', 'install', REPO] },
        { note: 'Done. In VS Code you can also run "Chat: Install Plugin from Source".' },
      ];
    case 'cursor':
      return [{ note: `Cursor installs plugins from its marketplace: import ${REPO} there (see the guide).` }];
    case 'gemini':
      return [{ gemini: true }];
  }
  if (!INSTALLER.has(target)) throw new Error(`no install plan for ${target}`);
  const steps = [];
  if (target === 'codex') {
    steps.push({ run: ['codex', 'plugin', 'marketplace', 'add', REPO], optional: true });
  }
  const install = ['install', '--target', target];
  if (scope) install.push('--scope', scope);
  if (force) install.push('--force');
  steps.push({ binary: true }, { loops: install });
  if (target === 'codex') steps.push({ note: 'Then install loops from the Codex desktop app and trust its hooks with /hooks.' });
  return steps;
}

// assetName is the release binary for a Node platform/arch, or '' when there
// is none.
export function assetName(platform, arch) {
  const os = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[platform];
  const cpu = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!os || !cpu) return '';
  return `loops-${os}-${cpu}${os === 'windows' ? '.exe' : ''}`;
}

// checksumFor finds asset's sha256 in a sha256sum-style checksums.txt.
export function checksumFor(text, asset) {
  for (const line of text.split('\n')) {
    const [sum, name] = line.trim().split(/\s+/);
    if (name === asset || name === `*${asset}`) return sum.toLowerCase();
  }
  return '';
}

// releaseTag is the tag to download: this package's version, or '' (latest)
// for an unpublished checkout.
export function releaseTag(version) {
  return !version || version === '0.0.0' ? '' : `v${version.replace(/^v/, '')}`;
}
