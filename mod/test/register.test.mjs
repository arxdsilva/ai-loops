// register.js against a stub $ over a temporary project: the hooks it adds,
// what the panes draw from real files, and that a command awaiting a
// refresh already in flight sees what that refresh read.
import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

globalThis.h = (type, props, ...kids) => ({ type, props, kids })
const { register } = await import('../register.js')
const flat = n => (typeof n === 'string' ? n : (n.kids || []).map(flat).join(''))

function project({ backlog, lane }) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'loops-mod-'))
  const wt = path.join(root, 'wt', 'issue-7-1')
  if (backlog) {
    fs.mkdirSync(path.join(root, '.loops', 'backlog'), { recursive: true })
    fs.writeFileSync(path.join(root, '.loops', 'backlog', 'state.json'), JSON.stringify({
      settings: { repo: 'o/r', lanes: 2, max_agents: 4, attempts: 2 },
      items: [{ number: 7, title: 'Fix login', status: 'running', attempts: 1, worktree: wt }, { number: 3, title: 'Next', status: 'queued', priority: 1 }],
    }))
  }
  if (lane) {
    fs.mkdirSync(path.join(wt, '.loops'), { recursive: true })
    fs.writeFileSync(path.join(wt, '.loops', 'status.json'), JSON.stringify({ active: true, state: { phase: 'qa', steps: 5, lanes: [], recent: [] } }))
  }
  return root
}

// mount registers the mod, runs session.start without awaiting its refresh
// (as the engine does), and returns the hooks and the panes opened.
async function mount(root) {
  const hooks = {}
  const opened = []
  let slow = true // the session.start refresh is still reading when the command runs
  const $ = {
    session: { root: async () => root },
    fs: {
      exists: async p => fs.existsSync(p),
      read: async p => { if (slow) await new Promise(r => setTimeout(r, 20)); return fs.readFileSync(p, 'utf8') },
    },
    ui: { open: async o => { opened.push(o.id); return { isPlaced: true } }, invalidate() {}, resolve: () => ({ Box: 'Box', Text: 'Text' }) },
    command: { register: async () => {} },
    clock: { every: () => ({ cancel() {} }) },
  }
  register((ev, m, fn) => { if (!fn) [fn, m] = [m, {}]; (hooks[ev] ||= []).push({ m, fn }) })
  await hooks['session.start'][0].fn($, {}, e => e)
  const command = name => hooks['command.run'].find(x => x.m.command === name).fn($)
  const pane = async id => (await hooks['ui.render'].find(x => x.m.requestId === id).fn($, {})).kids.map(flat).join('\n')
  return { command, pane, opened, fast: () => { slow = false } }
}

test('the backlog command sees the refresh already in flight', async () => {
  const m = await mount(project({ backlog: true, lane: true }))
  const r = await m.command('loops-backlog')
  assert.equal(r.text, 'loops backlog pane opened.')
  const out = await m.pane('loops-backlog')
  assert.match(out, /backlog o\/r/)
  assert.match(out, /#7 Fix login/)
  assert.match(out, /◉○○ {2}qa · agents 0\/0 · steps 5/)
  assert.match(out, /#3 P1 Next/)
  assert.ok(m.opened.includes('loops-backlog'), 'a running lane opens the pane by itself')
  assert.ok(!m.opened.includes('loops'), 'no run: the run pane stays shut')
})

test('a lane without a run yet, and no backlog at all', async () => {
  let m = await mount(project({ backlog: true }))
  await m.command('loops-backlog')
  assert.match(await m.pane('loops-backlog'), /no live loops run in the worktree/)
  m = await mount(project({}))
  assert.match((await m.command('loops-backlog')).text, /^No backlog in this project/)
  assert.deepEqual(m.opened.filter(id => id === 'loops-backlog'), ['loops-backlog'], 'only the command opened it')
  assert.match((await m.command('loops-pane')).text, /^No active loops run/)
  assert.match(await m.pane('loops'), /No active loops run/)
})
