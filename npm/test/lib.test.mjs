import { test } from 'node:test';
import assert from 'node:assert/strict';
import { assetName, checksumFor, parseArgs, plan, releaseTag, PLATFORMS } from '../lib.mjs';

test('no arguments installs into Claude Code', () => {
  assert.deepEqual(parseArgs([]), { cmd: 'install', opts: { target: 'claude', scope: '', force: false, binDir: '' } });
});

test('install options', () => {
  const p = parseArgs(['install', '--target', 'OpenCode', '--scope=user', '--force', '--bin-dir', '/b']);
  assert.deepEqual(p.opts, { target: 'opencode', scope: 'user', force: true, binDir: '/b' });
  assert.equal(parseArgs(['--target=amp']).opts.target, 'amp');
});

test('bad install arguments are rejected', () => {
  assert.throws(() => parseArgs(['--target', 'vim']), /unknown platform vim/);
  assert.throws(() => parseArgs(['--target']), /needs a value/);
  assert.throws(() => parseArgs(['--target', '--force']), /needs a value/);
  assert.throws(() => parseArgs(['install', 'x']), /unknown option x/);
});

test('bin installs only the binary', () => {
  assert.deepEqual(parseArgs(['bin']), { cmd: 'bin', opts: { binDir: '' } });
  assert.deepEqual(parseArgs(['bin', '--bin-dir', '/b']).opts, { binDir: '/b' });
  assert.deepEqual(parseArgs(['bin', '--bin-dir=/c']).opts, { binDir: '/c' });
  assert.equal(parseArgs(['bin', '-h']).cmd, 'help');
  assert.throws(() => parseArgs(['bin', '--bin-dir']), /needs a value/);
  assert.throws(() => parseArgs(['bin', 'x']), /unknown option x/);
});

test('help, version and pass-through', () => {
  assert.equal(parseArgs(['--help']).cmd, 'help');
  assert.equal(parseArgs(['install', '-h']).cmd, 'help');
  assert.equal(parseArgs(['--version']).cmd, 'version');
  assert.deepEqual(parseArgs(['report', '--open']), { cmd: 'run', rest: ['report', '--open'] });
});

test('claude plan adds the marketplace (or updates it) and installs the plugin', () => {
  const steps = plan({ target: 'claude', scope: '', force: false });
  assert.deepEqual(steps[0].run, ['claude', 'plugin', 'marketplace', 'add', 'arxdsilva/ai-loops']);
  assert.deepEqual(steps[0].fallback, ['claude', 'plugin', 'marketplace', 'update', 'ai-loops']);
  assert.deepEqual(steps[1].run, ['claude', 'plugin', 'install', 'loops@ai-loops', '--scope', 'user']);
  assert.deepEqual(plan({ target: 'claude', scope: 'project' })[1].run.slice(-1), ['project']);
});

test('installer platforms download the binary then run loops install', () => {
  const steps = plan({ target: 'opencode', scope: 'user', force: true });
  assert.deepEqual(steps, [{ binary: true }, { loops: ['install', '--target', 'opencode', '--scope', 'user', '--force'] }]);
  const codex = plan({ target: 'codex', scope: '', force: false });
  assert.equal(codex[0].optional, true);
  assert.deepEqual(codex[2], { loops: ['install', '--target', 'codex'] });
});

test('every platform has a plan', () => {
  for (const target of PLATFORMS) assert.ok(plan({ target, scope: '', force: false }).length > 0, target);
});

test('asset names match the release', () => {
  assert.equal(assetName('darwin', 'arm64'), 'loops-darwin-arm64');
  assert.equal(assetName('linux', 'x64'), 'loops-linux-amd64');
  assert.equal(assetName('win32', 'x64'), 'loops-windows-amd64.exe');
  assert.equal(assetName('freebsd', 'x64'), '');
  assert.equal(assetName('linux', 'ia32'), '');
});

test('checksum lookup', () => {
  const sums = 'AAA  loops-linux-amd64\nbbb *loops-darwin-arm64\n';
  assert.equal(checksumFor(sums, 'loops-linux-amd64'), 'aaa');
  assert.equal(checksumFor(sums, 'loops-darwin-arm64'), 'bbb');
  assert.equal(checksumFor(sums, 'loops-windows-amd64.exe'), '');
  assert.equal(checksumFor('', 'x'), '');
});

test('release tag follows the package version', () => {
  assert.equal(releaseTag('0.7.3'), 'v0.7.3');
  assert.equal(releaseTag('0.0.0'), '');
  assert.equal(releaseTag(''), '');
});
