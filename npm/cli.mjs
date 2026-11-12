#!/usr/bin/env node
// ai-loops: install the loops multi-agent workflow plugin with npx, or run
// the loops binary. Zero dependencies; Node 18+.
//
//   npx ai-loops                      # Claude Code (claude CLI): marketplace + plugin
//   npx ai-loops --target opencode    # any platform; see --help
//   npx ai-loops version              # any other loops command
//
// Binaries come from the GitHub release matching this package's version and
// are sha256-checked against its checksums.txt before they run.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { chmodSync, cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { homedir, tmpdir } from 'node:os';
import { join } from 'node:path';
import { assetName, checksumFor, parseArgs, plan, releaseTag, REPO, USAGE } from './lib.mjs';

const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8'));
const base = process.env.LOOPS_RELEASE_BASE || `https://github.com/${REPO}/releases/download`;

function die(msg) {
  console.error(`ai-loops: ${msg}`);
  process.exit(1);
}

function run(argv, { optional = false } = {}) {
  console.log(`$ ${argv.join(' ')}`);
  const r = spawnSync(argv[0], argv.slice(1), { stdio: 'inherit', shell: process.platform === 'win32' });
  if (r.error?.code === 'ENOENT') {
    if (optional) return false;
    die(`${argv[0]} not found on PATH; install it first`);
  }
  return r.status === 0;
}

async function fetchOk(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  return res;
}

async function resolveTag() {
  const tag = releaseTag(pkg.version);
  if (tag) return tag;
  const res = await fetch(`https://github.com/${REPO}/releases/latest`, { redirect: 'manual' });
  const loc = res.headers.get('location') || '';
  const latest = loc.split('/').pop();
  if (!latest?.startsWith('v')) die('could not resolve the latest release');
  return latest;
}

// download fetches the checksummed loops binary for tag to dest.
async function download(tag, dest) {
  const asset = assetName(process.platform, process.arch);
  if (!asset) die(`no prebuilt loops binary for ${process.platform}/${process.arch}; see the guide`);
  const sums = await (await fetchOk(`${base}/${tag}/checksums.txt`)).text();
  const want = checksumFor(sums, asset);
  if (!want) die(`checksums.txt for ${tag} has no ${asset}; refusing an unverified binary`);
  const bin = Buffer.from(await (await fetchOk(`${base}/${tag}/${asset}`)).arrayBuffer());
  const got = createHash('sha256').update(bin).digest('hex');
  if (got !== want) die(`checksum mismatch for ${asset} ${tag}; refusing it`);
  const tmp = `${dest}.new-${process.pid}`;
  writeFileSync(tmp, bin);
  chmodSync(tmp, 0o755);
  renameSync(tmp, dest);
}

const ext = process.platform === 'win32' ? '.exe' : '';

// cachedBinary returns a verified binary for tag in the shared loops cache
// (the same place the plugin's bin/loops shim keeps it).
async function cachedBinary(tag) {
  const dir = join(process.env.XDG_CACHE_HOME || join(homedir(), '.cache'), 'loops');
  const dest = join(dir, `loops-${tag.replace(/^v/, '')}${ext}`);
  if (!existsSync(dest)) {
    mkdirSync(dir, { recursive: true });
    await download(tag, dest);
  }
  return dest;
}

async function install(opts) {
  let loops = '';
  for (const step of plan(opts)) {
    if (step.note) console.log(step.note);
    else if (step.run) {
      if (!run(step.run, { optional: step.optional }) && !step.optional) {
        if (!step.fallback || !run(step.fallback)) die(`${step.run.join(' ')} failed`);
      }
    } else if (step.binary) {
      const tag = await resolveTag();
      const dir = opts.binDir || join(homedir(), '.local', 'bin');
      mkdirSync(dir, { recursive: true });
      loops = join(dir, `loops${ext}`);
      await download(tag, loops);
      console.log(`loops ${tag} installed to ${loops}`);
      if (!(process.env.PATH || '').split(process.platform === 'win32' ? ';' : ':').includes(dir)) {
        console.error(`ai-loops: warning: ${dir} is not on your PATH; add it so the hooks can find loops`);
      }
    } else if (step.loops) {
      if (!run([loops, ...step.loops])) die(`loops ${step.loops.join(' ')} failed`);
    } else if (step.gemini) {
      await gemini(await resolveTag());
    }
  }
}

// gemini installs platforms/gemini from the release's source archive, since
// Gemini CLI cannot install a repository subdirectory.
async function gemini(tag) {
  const tmp = mkdtempSync(join(tmpdir(), 'ai-loops-'));
  try {
    const tgz = join(tmp, 'src.tar.gz');
    writeFileSync(tgz, Buffer.from(await (await fetchOk(`https://codeload.github.com/${REPO}/tar.gz/refs/tags/${tag}`)).arrayBuffer()));
    if (!run(['tar', '-xzf', tgz, '-C', tmp])) die('could not unpack the source archive');
    const src = join(tmp, `ai-loops-${tag.replace(/^v/, '')}`, 'platforms', 'gemini');
    const dir = join(tmp, 'loops');
    cpSync(src, dir, { recursive: true });
    if (!run(['gemini', 'extensions', 'install', dir])) die('gemini extensions install failed');
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
}

let parsed;
try {
  parsed = parseArgs(process.argv.slice(2));
} catch (e) {
  die(`${e.message}\n${USAGE}`);
}
if (parsed.cmd === 'help') {
  console.log(USAGE);
} else if (parsed.cmd === 'version') {
  console.log(`ai-loops ${pkg.version}`);
} else if (parsed.cmd === 'install') {
  await install(parsed.opts).catch(e => die(e.message));
} else {
  const bin = await cachedBinary(await resolveTag()).catch(e => die(e.message));
  const r = spawnSync(bin, parsed.rest, { stdio: 'inherit' });
  process.exit(r.status ?? 1);
}
