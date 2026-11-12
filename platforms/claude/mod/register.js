import { buildRows, parseStatus } from './view.js'

const PANE = 'loops'
const TITLE = 'loops'

let status = null // parsed status, or {error}
let inFlight = false
let wasActive = false
let timer = null

// refresh reads .loops/status.json, the run state the plugin's hooks keep in
// the project (run.StatusDoc, the same document as `loops status --json`).
// It starts no program and sends nothing anywhere.
async function refresh($) {
  if (inFlight) return
  inFlight = true
  try {
    const root = await $.session.root()
    if (await $.fs.exists(root + '/.loops/status.json')) {
      status = parseStatus(await $.fs.read(root + '/.loops/status.json'))
    } else {
      status = { active: false } // no run has started in this project
    }
  } catch (err) {
    status = { error: 'loops status unavailable: ' + String((err && err.message) || err).split('\n')[0] }
  } finally {
    inFlight = false
  }
  const active = !!(status && status.active)
  if (active && !wasActive) {
    try { await $.ui.open({ id: PANE, title: TITLE }) } catch {}
  }
  wasActive = active
  try { $.ui.invalidate('ui.render') } catch {}
}

export const register = on => {
  on('session.start', async ($, e, next) => {
    try {
      await $.command.register({ name: 'loops-pane', description: 'Show the live loops run pane' })
    } catch {} // the pane still opens by itself when a run starts
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

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text } = $.ui.resolve(e)
    const rows = buildRows(status, Date.now())
    return h(Box, { flexDirection: 'column' }, ...rows.map(r =>
      h(Text, {}, ...r.map(s => h(Text, { dimColor: !!s.dim, bold: !!s.bold, color: s.color }, s.t)))))
  })
}
