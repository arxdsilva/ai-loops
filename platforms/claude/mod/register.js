import { buildRows, parseStatus } from './view.js'

const PANE = 'loops'
const TITLE = 'loops'

let status = null // parsed status, or {error}
let inFlight = false
let lastKick = 0
let wasActive = false
let timer = null

async function refresh($) {
  if (inFlight) return
  inFlight = true
  try {
    const res = await $.process.run([$.plugin.root + '/bin/loops', 'status', '--json'], { timeoutMs: 5000 })
    status = res.exitCode === 0 ? parseStatus(res.stdout) : { error: 'loops status failed: ' + String(res.stderr || res.exitCode).split('\n')[0] }
  } catch (err) {
    status = { error: 'loops binary unavailable: ' + String((err && err.message) || err).split('\n')[0] }
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

async function debounced($) {
  const now = Date.now()
  if (now - lastKick < 500) return
  lastKick = now
  await refresh($)
}

export const register = on => {
  on('session.start', async ($, e, next) => {
    await $.command.register({ name: 'loops', description: 'Show the live loops run pane' })
    if (timer) timer.cancel()
    timer = $.clock.every(3000, () => { void refresh($) })
    void refresh($)
    return next(e)
  })

  on('command.run', { command: 'loops' }, async $ => {
    await refresh($)
    const r = await $.ui.open({ id: PANE, title: TITLE })
    if (!(status && status.active)) return { text: 'No active loops run — start one with /loops:work <ticket>' }
    return { text: r && r.isPlaced === false ? 'loops pane could not be placed (terminal too narrow?).' : 'loops pane opened.' }
  })

  on('classic.PostToolUse', async ($, e, next) => { void debounced($); return next(e) })
  on('classic.SubagentStart', async ($, e, next) => { void debounced($); return next(e) })
  on('classic.SubagentStop', async ($, e, next) => { void debounced($); return next(e) })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text } = $.ui.resolve(e)
    const rows = buildRows(status, Date.now())
    return h(Box, { flexDirection: 'column' }, ...rows.map(r =>
      h(Text, {}, ...r.map(s => h(Text, { dimColor: !!s.dim, bold: !!s.bold, color: s.color }, s.t)))))
  })
}
