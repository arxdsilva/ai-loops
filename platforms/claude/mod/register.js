import { buildRows, parseStatus } from './view.js'
import { buildBacklogRows, parseBacklog, runningCount, runningWorktrees } from './backlog.js'

const PANE = 'loops'
const TITLE = 'loops'
const BACKLOG_PANE = 'loops-backlog'
const BACKLOG_TITLE = 'loops backlog'

let status = null // parsed status, or {error}
let backlog // parsed backlog state, {error}, null (no backlog) or undefined (not read yet)
let laneDocs = new Map() // running lane's issue number -> its worktree's parsed status.json
let inFlight = null // the refresh in flight
let wasActive = false
let wasRunning = false
let timer = null

// readBacklog reads .loops/backlog/state.json, then each running lane's
// <worktree>/.loops/status.json (the lane's own hooks keep it current). A
// worktree without one is a lane starting or wrapping up, not an error.
async function readBacklog($, root) {
  const path = root + '/.loops/backlog/state.json'
  if (!(await $.fs.exists(path))) return { st: null, docs: new Map() }
  const st = parseBacklog(await $.fs.read(path))
  const docs = new Map()
  for (const { number, dir } of runningWorktrees(st, root)) {
    try {
      if (await $.fs.exists(dir + '/.loops/status.json')) docs.set(number, parseStatus(await $.fs.read(dir + '/.loops/status.json')))
    } catch {} // a worktree removed under us: show the lane without its run
  }
  return { st, docs }
}

// refresh reads .loops/status.json, the run state the plugin's hooks keep in
// the project (run.StatusDoc, the same document as `loops status --json`),
// and, when the project runs a backlog, .loops/backlog/state.json and each
// running lane's status.json in its worktree. It starts no program and sends
// nothing anywhere.
// A refresh already in flight is shared, so a command awaiting it sees what
// it read rather than the state before it.
function refresh($) {
  if (!inFlight) inFlight = read($).finally(() => { inFlight = null })
  return inFlight
}

async function read($) {
  try {
    const root = await $.session.root()
    if (await $.fs.exists(root + '/.loops/status.json')) {
      status = parseStatus(await $.fs.read(root + '/.loops/status.json'))
    } else {
      status = { active: false } // no run has started in this project
    }
  } catch (err) {
    status = { error: 'loops status unavailable: ' + String((err && err.message) || err).split('\n')[0] }
  }
  try {
    const { st, docs } = await readBacklog($, await $.session.root())
    backlog = st
    laneDocs = docs
  } catch (err) {
    backlog = { error: 'loops backlog unavailable: ' + String((err && err.message) || err).split('\n')[0] }
    laneDocs = new Map()
  }
  const active = !!(status && status.active)
  if (active && !wasActive) {
    try { await $.ui.open({ id: PANE, title: TITLE }) } catch {}
  }
  wasActive = active
  const running = runningCount(backlog) > 0 // the pane opens by itself when a lane starts
  if (running && !wasRunning) {
    try { await $.ui.open({ id: BACKLOG_PANE, title: BACKLOG_TITLE }) } catch {}
  }
  wasRunning = running
  try { $.ui.invalidate('ui.render') } catch {}
}

export const register = on => {
  on('session.start', async ($, e, next) => {
    try {
      await $.command.register({ name: 'loops-pane', description: 'Show the live loops run pane' })
      await $.command.register({ name: 'loops-backlog', description: 'Show the live loops backlog pane: lanes, queue, outcomes' })
    } catch {} // the panes still open by themselves when a run or a lane starts
    if (timer) timer.cancel()
    timer = $.clock.every(2000, () => { void refresh($) })
    void refresh($)
    return next(e)
  })

  on('command.run', { command: 'loops-pane' }, async $ => { // '/loops' is a name Claude Code ships
    try {
      await refresh($)
      const r = await $.ui.open({ id: PANE, title: TITLE })
      if (!(status && status.active)) return { text: 'No active loops run — start one with /loops:work <ticket>' }
      return { text: r && r.isPlaced === false ? 'loops pane could not be placed (terminal too narrow?).' : 'loops pane opened.' }
    } catch (err) {
      return { text: 'loops pane unavailable: ' + String((err && err.message) || err).split('\n')[0] }
    }
  })

  on('command.run', { command: 'loops-backlog' }, async $ => {
    try {
      await refresh($)
      const r = await $.ui.open({ id: BACKLOG_PANE, title: BACKLOG_TITLE })
      if (!backlog) return { text: 'No backlog in this project — start one with /loops:backlog --human <login>' }
      return { text: r && r.isPlaced === false ? 'loops backlog pane could not be placed (terminal too narrow?).' : 'loops backlog pane opened.' }
    } catch (err) {
      return { text: 'loops backlog pane unavailable: ' + String((err && err.message) || err).split('\n')[0] }
    }
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => draw($, e, buildRows(status, Date.now())))
  on('ui.render', { component: 'Pane', requestId: BACKLOG_PANE }, async ($, e) => draw($, e, buildBacklogRows(backlog, laneDocs, Date.now())))
}

function draw($, e, rows) {
  const { Box, Text } = $.ui.resolve(e)
  return h(Box, { flexDirection: 'column' }, ...rows.map(r =>
    h(Text, {}, ...r.map(s => h(Text, { dimColor: !!s.dim, bold: !!s.bold, color: s.color }, s.t)))))
}
