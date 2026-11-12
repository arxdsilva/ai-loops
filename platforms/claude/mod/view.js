// Pure view helpers: loops status JSON -> rows of styled segments.
// A row is an array of segments {t, dim?, bold?, color?}. No Claude APIs here.

export const PHASES = ['intake', 'research', 'plan', 'implement', 'qa', 'product', 'reflect']
const PHASE_NAME = { intake: 'Intake', research: 'Research', plan: 'Plan', implement: 'Implement', qa: 'QA', product: 'Product', reflect: 'Reflect' }
export const GATES = ['build', 'gofmt', 'vet', 'staticcheck', 'arch', 'test']
export const IDLE_TEXT = 'No active loops run — start one with /loops:work <ticket>'

const seg = (t, o = {}) => ({ t, ...o })
const trunc = (s, n) => { s = String(s ?? ''); return s.length > n ? s.slice(0, n - 1) + '…' : s }
const pad = (s, n) => { s = trunc(s, n); return s + ' '.repeat(Math.max(0, n - s.length)) }

export function fmtDuration(ms) {
  if (!Number.isFinite(ms) || ms < 0) return '0s'
  const s = Math.floor(ms / 1000)
  if (s < 60) return s + 's'
  const m = Math.floor(s / 60)
  if (m < 60) return m + 'm' + String(s % 60).padStart(2, '0') + 's'
  return Math.floor(m / 60) + 'h' + String(m % 60).padStart(2, '0') + 'm'
}

const ts = x => { const n = Date.parse(x); return Number.isFinite(n) ? n : NaN }

export function parseStatus(stdout) {
  try {
    const v = JSON.parse(stdout)
    if (!v || typeof v !== 'object') return { error: 'unexpected loops status output' }
    return v
  } catch {
    return { error: 'could not parse `loops status --json` output' }
  }
}

export function header(st, now) {
  const end = st.ended ? ts(st.updated) : now
  const el = fmtDuration(end - ts(st.started))
  const row = [seg(st.ended ? '■ ' : '▶ ', { color: st.ended ? undefined : 'green' }), seg(trunc(st.title || st.id, 50), { bold: true })]
  if (st.ticket) row.push(seg(' · ' + st.ticket))
  row.push(seg(' · ' + el))
  if (st.ended) {
    row.push(seg(' · ' + (st.outcome || 'ended'), { color: st.outcome === 'abandoned' || st.outcome === 'failed' ? 'red' : 'green' }))
    if (st.pr) row.push(seg(' · ' + st.pr))
  } else if (st.pr) row.push(seg(' · ' + st.pr))
  return row
}

export function phaseStrip(st) {
  const visited = new Set((st.phase_log || []).map(p => p.phase))
  const idx = PHASES.indexOf(st.phase)
  const loopsOf = ph => {
    const lb = (st.loop_backs || [])
    if (ph === 'plan') return st.plan_rounds > 1 ? ' r' + st.plan_rounds : ''
    if (ph === 'qa') { const n = lb.filter(l => l.phase === 'qa').length; return n ? ' ↺' + n : '' }
    if (ph === 'product') { const n = lb.filter(l => l.phase === 'product').length; return n ? ' ↺' + n : '' }
    return ''
  }
  const row = []
  PHASES.forEach((ph, i) => {
    if (i) row.push(seg(' › ', { dim: true }))
    const label = PHASE_NAME[ph] + loopsOf(ph)
    const cur = ph === st.phase && !st.ended
    const done = st.ended || i < idx || (visited.has(ph) && i < idx)
    if (cur) row.push(seg('[' + label + ']', { bold: true, color: 'cyan' }))
    else if (done) row.push(seg('✓' + label, { dim: true }))
    else row.push(seg(label, { dim: true }))
  })
  return row
}

export function fmtTokens(n) {
  n = Number(n) || 0
  if (n >= 1e6) return String(Math.floor(n / 1e5) / 10) + 'M'
  if (n >= 1e3) return Math.floor(n / 1e3) + 'k'
  return String(n)
}

// Overrides count, per-phase tokens vs limits, and the budget pause marker.
// Empty when the run has none of them.
export function limitRows(st) {
  const rows = []
  const ov = st.overrides || []
  if (ov.length) {
    const last = ov[ov.length - 1]
    rows.push([seg('overrides ' + ov.length, { color: 'yellow' }), seg(` · last: ${trunc(last.loops_rule, 30)} → ${trunc(last.project_rule, 40)}`, { dim: true })])
  }
  const phases = (st.limits && st.limits.phases) || {}
  const parts = PHASES.filter(p => phases[p] && phases[p].max_tokens > 0)
    .map(p => `${PHASE_NAME[p]} ${fmtTokens((st.phase_tokens || {})[p])}/${fmtTokens(phases[p].max_tokens)} tok`)
  if (parts.length) rows.push([seg('limits ', { dim: true }), seg(parts.join('  '))])
  const b = (st.budgets || []).at(-1)
  if (b && b.phase === st.phase) {
    const log = st.phase_log || []
    const since = log.length ? ts(log[log.length - 1].at) : NaN
    const spawned = (st.lanes || []).some(l => l.phase === st.phase && ts(l.started) > ts(b.at))
    if (!(ts(b.at) <= since) && !spawned && !(ts(b.at) <= ts(st.limits_at))) rows.push([seg('⏸ budget: ' + trunc(b.reason || b.limit, 70), { color: 'yellow' })])
  }
  return rows
}

export function laneRows(st) {
  const lanes = st.lanes || []
  const tok = lanes.some(l => l.tokens > 0)
  const out = [[seg(pad('lane', 30) + pad('model', 8) + pad('wave', 5) + pad('state', 8) + pad('steps', 6) + (tok ? pad('tokens', 8) : '') + 'QA', { dim: true })]]
  for (const l of lanes.slice(-10)) {
    const name = (l.label || l.agent || '') + '|' + (l.role || '')
    const state = l.running ? 'running' : l.ok === false ? 'failed' : 'done'
    const qa = l.qa === 'VERIFIED' ? '✓' : l.qa === 'FAILED' ? '✗' : '-'
    const qaTxt = qa + (l.qa_rounds > 1 ? '↺' + (l.qa_rounds - 1) : '')
    out.push([
      seg(pad(name, 30)),
      seg(pad(l.model || '-', 8), { dim: true }),
      seg(pad(l.wave != null && l.wave !== 0 ? 'w' + l.wave : (l.wave === 0 ? 'w0' : '-'), 5), { dim: true }),
      seg(pad(state, 8), { color: l.running ? 'yellow' : l.ok === false ? 'red' : 'green' }),
      seg(pad(String(l.steps ?? 0) + (l.errors ? '!' + l.errors : ''), 6)),
      ...(tok ? [seg(pad(l.tokens > 0 ? fmtTokens(l.tokens) : '', 8), { dim: true })] : []),
      seg(qaTxt, { color: qa === '✓' ? 'green' : qa === '✗' ? 'red' : undefined }),
    ])
  }
  if (lanes.length > 10) out.push([seg('… ' + (lanes.length - 10) + ' earlier lanes', { dim: true })])
  if (!lanes.length) return [[seg('no agent lanes yet', { dim: true })]]
  return out
}

export function gatesLine(st) {
  const by = new Map((st.gates || []).map(g => [g.name, g]))
  const names = [...GATES, ...[...by.keys()].filter(n => !GATES.includes(n) && n !== 'all')]
  const row = [seg('gates ', { dim: true })]
  for (const n of names) {
    const g = by.get(n)
    row.push(seg(n + (g ? (g.ok ? '✓' : '✗') : '-') + ' ', { color: g ? (g.ok ? 'green' : 'red') : undefined, dim: !g }))
  }
  if (by.get('all')) row.push(seg(by.get('all').ok ? 'all✓' : 'all✗', { color: by.get('all').ok ? 'green' : 'red' }))
  if (st.source_dirty || st.go_dirty) row.push(seg(' (source dirty)', { color: 'yellow' }))
  return row
}

export function verdictRows(st) {
  const latest = new Map()
  for (const v of st.verdicts || []) if (v.phase === 'product' || /^AC/i.test(v.name || '')) latest.set(v.name, v)
  const total = latest.size
  const ok = [...latest.values()].filter(v => v.status === 'VERIFIED').length
  const rows = [[seg('ACs ', { dim: true }), seg(`${ok}/${total} verified`, { color: total && ok === total ? 'green' : undefined })]]
  const lbs = (st.loop_backs || []).slice(-3)
  for (const l of lbs) {
    rows.push([seg('↺ ', { color: 'yellow' }), seg(`${l.from}→${l.to}${l.round ? ' r' + l.round : ''}: ${trunc(l.reason || '', 60)}`)])
  }
  return rows
}

export function stepRows(st, n = 8) {
  const steps = (st.recent || []).slice(-n)
  if (!steps.length) return [[seg('no steps yet', { dim: true })]]
  return steps.map(s => [
    seg('[' + (s.role || 'lead') + '] ', { dim: true }),
    seg(s.tool + ' '),
    seg(trunc(s.target || '', 44) + ' '),
    seg(s.ok ? '✓' : '✗', { color: s.ok ? 'green' : 'red' }),
    seg(s.ms ? ' ' + s.ms + 'ms' : '', { dim: true }),
  ])
}

// status: parsed `loops status --json`, or {error}. now: ms epoch.
export function buildRows(status, now = Date.now()) {
  if (!status) return [[seg('loading…', { dim: true })]]
  if (status.error) return [[seg(trunc(status.error, 100), { color: 'red' })]]
  if (!status.active || !status.state) return [[seg(IDLE_TEXT, { dim: true })]]
  const st = status.state
  return [
    header(st, now),
    phaseStrip(st),
    ...limitRows(st),
    [seg('')],
    ...laneRows(st),
    [seg('')],
    gatesLine(st),
    ...verdictRows(st),
    [seg('')],
    ...stepRows(st),
  ]
}

export const toPlain = rows => rows.map(r => r.map(s => s.t).join('')).join('\n')
