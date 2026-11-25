// Pure view helpers for the backlog pane (specs/BACKLOG.md §8): the backlog
// state (.loops/backlog/state.json) and each running lane's run status
// (<worktree>/.loops/status.json) -> rows of styled segments, the same shape
// view.js draws. It mirrors `loops backlog watch`. No Claude APIs here.

import { PHASES, fmtDuration } from './view.js'

export const BACKLOG_IDLE = 'No backlog in this project — start one with /loops:backlog --human <login>'
const STATUSES = ['queued', 'running', 'done', 'failed', 'human', 'skipped']
const FINISHED = new Set(['done', 'failed', 'human', 'skipped'])
const DEFAULT_MAX_PRS = 5
const SHOWN = 6
const QUIET_MS = 10 * 60 * 1000

const seg = (t, o = {}) => ({ t, ...o })
const trunc = (s, n) => { s = String(s ?? ''); return s.length > n ? s.slice(0, n - 1) + '…' : s }
const ts = x => { const n = Date.parse(x); return Number.isFinite(n) && n > 0 ? n : NaN }
const prio = p => (Number.isInteger(p) && p < 10 ? String(p) : '-')

export function parseBacklog(text) {
  try {
    const v = JSON.parse(text)
    if (!v || typeof v !== 'object' || !Array.isArray(v.items)) return { error: 'unexpected .loops/backlog/state.json content' }
    return v
  } catch {
    return { error: 'could not parse .loops/backlog/state.json' }
  }
}

// The worktrees whose status the pane reads: the running lanes'.
export function runningWorktrees(st, root) {
  if (!st || !Array.isArray(st.items)) return []
  return st.items.filter(it => it.status === 'running' && it.worktree)
    .map(it => ({ number: it.number, dir: it.worktree.startsWith('/') ? it.worktree : root + '/' + it.worktree }))
}

export const runningCount = st => (st && Array.isArray(st.items) ? st.items.filter(it => it.status === 'running').length : 0)

// progress summarises a lane's status document (run.StatusDoc), or null when
// it holds no live run. Mirrors backlog.NewProgress.
export function progress(doc) {
  const s = doc && doc.active && doc.state
  if (!s) return null
  const lanes = s.lanes || []
  const gates = s.gates || []
  const acs = new Map()
  for (const v of s.verdicts || []) if (v.phase === 'product' || /^AC/.test(v.name || '')) acs.set(v.name, v.status)
  const last = (s.recent || []).at(-1)
  return {
    phase: s.phase, ended: !!s.ended, outcome: s.outcome || '', pr: s.pr || '', updated: s.updated,
    steps: s.steps || 0, errors: s.errors || 0, agents: lanes.length, busy: lanes.filter(l => l.running).length,
    gates: gates.length, gatesOK: gates.filter(g => g.ok).length,
    acs: acs.size, acsOK: [...acs.values()].filter(x => x === 'VERIFIED').length,
    rounds: Math.max((s.plan_rounds || 0) - 1, 0) + (s.loop_backs || []).length,
    last: last ? [last.role || 'lead', last.tool, last.target].filter(Boolean).join(' ') : '',
  }
}

const prCap = set => { const m = set.max_prs || 0; return m < 0 ? 0 : m || DEFAULT_MAX_PRS }

// prKey identifies a PR by its number, as backlog's prKey does, so two
// spellings of one PR's URL count once.
function prKey(url) {
  const u = String(url || '').trim().replace(/\/$/, '').toLowerCase()
  const m = u.match(/\/pull\/(\d+)$/)
  return m ? 'pull/' + Number(m[1]) : u
}

function lanePRs(it) {
  const keys = new Set((it.lane_prs || []).map(p => prKey(p.url)))
  if (it.pr) keys.add(prKey(it.pr))
  return keys.size
}

function headRows(st, now) {
  const set = st.settings || {}
  const counts = Object.fromEntries(STATUSES.map(s => [s, 0]))
  let prs = 0
  for (const it of st.items) { counts[it.status] = (counts[it.status] || 0) + 1; prs += lanePRs(it) }
  const head = [seg('backlog ' + (set.repo || '?'), { bold: true })]
  if (set.base) head.push(seg(' · base ' + set.base))
  if (set.human) head.push(seg(' · human @' + set.human))
  const synced = ts(st.synced)
  head.push(seg('  ' + (Number.isFinite(synced) ? 'synced ' + fmtDuration(now - synced) + ' ago' : 'never synced'), { dim: true }))
  const color = { running: 'yellow', done: 'green', failed: 'red', human: 'red' }
  const countRow = []
  STATUSES.forEach((s, i) => {
    if (i) countRow.push(seg(' · ', { dim: true }))
    countRow.push(seg(s + ' ' + counts[s], counts[s] ? { color: color[s] } : { dim: true }))
  })
  const cap = prCap(set)
  const pending = (st.pending_findings || []).length
  const caps = `lanes ${counts.running}/${set.lanes || 0} × ${set.max_agents || 0} agents · lane PRs ${prs}` +
    (cap ? ` (≤${cap} open)` : '') + ` · findings ${(st.findings || []).length} filed` + (pending ? `, ${pending} pending` : '')
  return { counts, rows: [head, countRow, [seg(caps, { dim: true })]] }
}

function strip(p) {
  const cur = PHASES.indexOf(p.phase)
  return PHASES.map((_, i) => (i < cur || (p.ended && i === cur))
    ? seg('●', { color: 'green' })
    : i === cur ? seg('◉', { color: 'yellow' }) : seg('○', { dim: true }))
}

function laneRows(it, doc, set, now) {
  const started = ts(it.started)
  const rows = [[
    seg('▶ ', { color: 'yellow' }), seg('#' + it.number + ' ' + trunc(it.title, 60), { bold: true }),
    seg(`  attempt ${it.attempts || 0}/${set.attempts || 0}` + (Number.isFinite(started) ? ' · ' + fmtDuration(now - started) : ''), { dim: true }),
  ]]
  const p = progress(doc)
  if (!p) return [...rows, [seg('   no live loops run in the worktree (starting, or wrapping up)', { dim: true })]]
  const line = [seg('   '), ...strip(p), seg('  ' + p.phase), seg(` · agents ${p.busy}/${p.agents} · steps ${p.steps}`)]
  if (p.errors) line.push(seg('!' + p.errors, { color: 'red' }))
  if (p.gates) line.push(seg(' · '), seg(`gates ${p.gatesOK}/${p.gates}`, { color: p.gatesOK === p.gates ? 'green' : 'red' }))
  if (p.acs) line.push(seg(` · ACs ${p.acsOK}/${p.acs}`))
  if (p.rounds) line.push(seg(' · ↺' + p.rounds, { color: 'yellow' }))
  rows.push(line)
  const upd = ts(p.updated)
  const pr = p.pr ? ' · PR ' + p.pr : ''
  if (p.ended) rows.push([seg(('   run ended ' + p.outcome).trimEnd() + pr + ': the lane is wrapping up', { dim: true })])
  else if (Number.isFinite(upd) && now - upd > QUIET_MS) {
    rows.push([seg(`   quiet for ${fmtDuration(now - upd)}: last ${trunc(p.last, 60)} (a gone lane is recorded by loops backlog sync)`, { color: 'yellow' })])
  } else if (p.last) rows.push([seg('   last: ' + trunc(p.last, 70) + pr, { dim: true })])
  return rows
}

const MARK = { done: ['✓', 'green'], failed: ['✗', 'red'], human: ['⚑', 'red'] }

function recentRow(it) {
  const [m, color] = MARK[it.status] || ['–', undefined]
  let text = trunc(it.title, 50)
  if (it.status === 'done' && it.pr) text += '  ' + it.pr
  else text += '  (' + it.status + (it.note ? ': ' + trunc(it.note, 60) : '') + ')'
  return [seg('  '), seg(m, color ? { color } : { dim: true }), seg(' #' + it.number + ' '), seg(text)]
}

// st: parsed state.json, or {error}, or null (no backlog). lanes: Map of
// issue number -> parsed status document of its worktree. now: ms epoch.
export function buildBacklogRows(st, lanes = new Map(), now = Date.now()) {
  if (st === undefined) return [[seg('loading…', { dim: true })]]
  if (!st) return [[seg(BACKLOG_IDLE, { dim: true })]]
  if (st.error) return [[seg(trunc(st.error, 100), { color: 'red' })]]
  const set = st.settings || {}
  const { counts, rows } = headRows(st, now)
  rows.push([seg('')])
  const running = st.items.filter(it => it.status === 'running')
  if (!running.length) rows.push([seg('no lanes running', { dim: true })])
  for (const it of running) rows.push(...laneRows(it, lanes.get(it.number), set, now))

  const queued = st.items.filter(it => it.status === 'queued')
    .sort((a, b) => (a.priority - b.priority) || (a.number - b.number))
  if (queued.length) {
    rows.push([seg('')], [seg(`next (${counts.queued} queued)`, { dim: true })])
    for (const it of queued.slice(0, SHOWN)) rows.push([seg(`  #${it.number} `), seg('P' + prio(it.priority) + ' ', { dim: true }), seg(trunc(it.title, 70))])
  }
  const recent = st.items.filter(it => FINISHED.has(it.status))
    .sort((a, b) => ((ts(b.updated) || 0) - (ts(a.updated) || 0)) || (b.number - a.number))
  if (recent.length) {
    rows.push([seg('')], [seg('recent', { dim: true })])
    for (const it of recent.slice(0, SHOWN)) rows.push(recentRow(it))
  }
  return rows
}
