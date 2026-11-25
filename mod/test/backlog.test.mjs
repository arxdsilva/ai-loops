import test from 'node:test'
import assert from 'node:assert/strict'
import { buildBacklogRows, parseBacklog, progress, runningCount, runningWorktrees, BACKLOG_IDLE } from '../backlog.js'
import { toPlain } from '../view.js'

const NOW = Date.parse('2026-11-25T12:00:00Z')
const at = mins => new Date(NOW - mins * 60000).toISOString()

const state = {
  version: 1,
  settings: { repo: 'o/r', base: 'main', human: 'hum', lanes: 2, max_agents: 4, attempts: 2, max_prs: 0 },
  synced: at(3),
  items: [
    { number: 7, title: 'Fix login', status: 'running', attempts: 1, worktree: '/wt/issue-7-1', started: at(25) },
    { number: 8, title: 'Slow one', status: 'running', attempts: 2, worktree: 'rel/issue-8-2', started: at(90) },
    { number: 9, title: 'Ghost', status: 'running', attempts: 1, worktree: '/wt/issue-9-1' },
    { number: 3, title: 'Next one', status: 'queued', priority: 1 },
    { number: 2, title: 'Urgent', status: 'queued', priority: 0 },
    { number: 4, title: 'Unranked', status: 'queued', priority: 10 },
    { number: 5, title: 'Shipped', status: 'done', pr: 'https://x/pull/5', lane_prs: [{ url: 'https://X/pull/5/' }], updated: at(30) },
    { number: 6, title: 'Stuck', status: 'human', note: 'needs a product decision', lane_prs: [{ url: 'https://x/pull/4' }], updated: at(5) },
  ],
  findings: [{ key: 'a', title: 'A', url: 'u' }],
  pending_findings: [{ title: 'b', body: '' }],
}

const live = (s, mins = 0) => ({ active: true, state: { updated: at(mins), ...s } })

const lanes = new Map([
  [7, live({ phase: 'implement', steps: 142, errors: 2, plan_rounds: 2,
    lanes: [{ running: true }, { running: true }, { running: true }, {}, {}],
    gates: [{ name: 'build', ok: true }, { name: 'vet', ok: true }, { name: 'test', ok: false }],
    recent: [{ role: 'go-engineer', tool: 'Edit', target: 'x.go' }] })],
  [8, live({ phase: 'qa', recent: [{ tool: 'Bash', target: 'go test' }] }, 60)],
])

test('idle, loading and errors', () => {
  assert.equal(toPlain(buildBacklogRows(null, new Map(), NOW)), BACKLOG_IDLE)
  assert.equal(toPlain(buildBacklogRows(undefined, new Map(), NOW)), 'loading…')
  assert.match(toPlain(buildBacklogRows(parseBacklog('nope'), new Map(), NOW)), /could not parse/)
  assert.match(toPlain(buildBacklogRows(parseBacklog('{"x":1}'), new Map(), NOW)), /unexpected/)
})

test('the board', () => {
  const out = toPlain(buildBacklogRows(state, lanes, NOW))
  for (const want of [
    'backlog o/r · base main · human @hum  synced 3m00s ago',
    'queued 3 · running 3 · done 1 · failed 0 · human 1 · skipped 0',
    'lanes 3/2 × 4 agents · lane PRs 2 (≤5 open) · findings 1 filed, 1 pending',
    '▶ #7 Fix login  attempt 1/2 · 25m00s',
    '●●●◉○○○  implement · agents 3/5 · steps 142!2 · gates 2/3 · ↺1',
    'last: go-engineer Edit x.go',
    'quiet for 1h00m: last lead Bash go test',
    '▶ #9 Ghost  attempt 1/2',
    'no live loops run in the worktree (starting, or wrapping up)',
    'next (3 queued)',
    '✓ #5 Shipped  https://x/pull/5',
    '⚑ #6 Stuck  (human: needs a product decision)',
  ]) assert.ok(out.includes(want), `missing ${JSON.stringify(want)} in:\n${out}`)
  // queue in lane order: priority, then number; recent newest first
  assert.ok(out.indexOf('#2 P0 Urgent') < out.indexOf('#3 P1 Next one') && out.indexOf('#3 P1') < out.indexOf('#4 P- Unranked'), out)
  assert.ok(out.indexOf('#6 Stuck') < out.indexOf('#5 Shipped'), out)
})

test('empty backlog and no PR cap', () => {
  const out = toPlain(buildBacklogRows({ settings: { repo: 'o/r', lanes: 2, max_agents: 3, max_prs: -1 }, items: [] }, new Map(), NOW))
  assert.match(out, /never synced/)
  assert.match(out, /no lanes running/)
  assert.match(out, /lane PRs 0 · findings 0 filed/)
  assert.doesNotMatch(out, /next \(|recent|≤/)
})

test('an ended run is wrapping up', () => {
  const st = { settings: {}, items: [{ number: 1, title: 'T', status: 'running', worktree: '/w' }] }
  const out = toPlain(buildBacklogRows(st, new Map([[1, live({ phase: 'reflect', ended: true, outcome: 'done', pr: 'https://x/pull/9' })]]), NOW))
  assert.match(out, /●●●●●●● {2}reflect/)
  assert.match(out, /run ended done · PR https:\/\/x\/pull\/9: the lane is wrapping up/)
})

test('progress mirrors backlog.NewProgress', () => {
  assert.equal(progress(null), null)
  assert.equal(progress({ active: false }), null)
  const p = progress(live({ phase: 'product', verdicts: [
    { name: 'AC1', status: 'FAILED', phase: 'product' }, { name: 'AC1', status: 'VERIFIED', phase: 'product' },
    { name: 'AC2', status: 'FAILED', phase: 'product' }, { name: 'W1:C1', status: 'VERIFIED', phase: 'qa' }],
  loop_backs: [{}], plan_rounds: 1 }))
  assert.deepEqual([p.acsOK, p.acs, p.rounds, p.last], [1, 2, 1, ''])
})

test('running worktrees and count', () => {
  assert.deepEqual(runningWorktrees(state, '/repo'), [
    { number: 7, dir: '/wt/issue-7-1' }, { number: 8, dir: '/repo/rel/issue-8-2' }, { number: 9, dir: '/wt/issue-9-1' }])
  assert.equal(runningCount(state), 3)
  assert.equal(runningCount(null), 0)
  assert.deepEqual(runningWorktrees({ error: 'x' }, '/r'), [])
})
