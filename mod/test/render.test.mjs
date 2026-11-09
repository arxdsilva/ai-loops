import test from 'node:test'
import assert from 'node:assert/strict'
import { buildRows, toPlain, parseStatus, fmtDuration, IDLE_TEXT } from '../view.js'

const T0 = '2026-11-09T10:00:00Z'
const NOW = Date.parse('2026-11-09T10:12:30Z')
const base = { id: 'r1', title: 'Add rate limiter', ticket: 'GH-42', started: T0, updated: T0, phase_log: [], lanes: [], gates: [], verdicts: [], loop_backs: [], recent: [], plan_rounds: 1, steps: 0 }

const mid = { active: true, dir: '/x/.loops/runs/r1', state: { ...base,
  phase: 'implement', plan_rounds: 2,
  phase_log: ['intake', 'research', 'plan', 'implement'].map(p => ({ phase: p, at: T0 })),
  lanes: [
    { agent: 'a1', label: 'W1-domain', role: 'domain-engineer', model: 'sonnet', wave: 1, running: false, ok: true, steps: 14, qa: 'FAILED', qa_rounds: 2 },
    { agent: 'a2', label: 'W1-store', role: 'store-engineer', model: 'sonnet', wave: 1, running: true, steps: 6, errors: 1 },
  ],
  gates: [{ name: 'build', ok: true }, { name: 'gofmt', ok: true }, { name: 'vet', ok: true }, { name: 'arch', ok: true }, { name: 'test', ok: false }],
  loop_backs: [{ from: 'qa-verifier', to: 'domain-engineer', phase: 'qa', round: 1, reason: 'missing nil check' }],
  recent: [{ role: 'domain-engineer', tool: 'Edit', target: 'internal/domain/x.go', ok: true, ms: 12 }, { role: 'store-engineer', tool: 'Bash', target: 'go test ./...', ok: false, ms: 900 }],
} }

const ended = { active: true, state: { ...base, phase: 'reflect', ended: true, outcome: 'success', pr: 'https://github.com/o/r/pull/7', updated: '2026-11-09T10:30:00Z',
  verdicts: [{ name: 'AC1', status: 'VERIFIED', phase: 'product' }, { name: 'AC2', status: 'VERIFIED', phase: 'product' }] } }

test('idle', () => {
  assert.equal(toPlain(buildRows({ active: false }, NOW)), IDLE_TEXT)
})
test('errors never throw', () => {
  assert.match(toPlain(buildRows(parseStatus('not json'), NOW)), /could not parse/)
  assert.match(toPlain(buildRows({ error: 'binary missing' }, NOW)), /binary missing/)
})
test('mid-implementation with QA failure', () => {
  const out = toPlain(buildRows(mid, NOW))
  console.log(out)
  assert.match(out, /Add rate limiter .* GH-42 .* 12m30s/)
  assert.match(out, /\[Implement\]/)
  assert.match(out, /Plan r2/)
  assert.match(out, /QA ↺1/)
  assert.match(out, /W1-domain\|domain-engineer/)
  assert.match(out, /test✗/)
  assert.match(out, /staticcheck-/)
  assert.match(out, /qa-verifier→domain-engineer r1: missing nil check/)
  assert.match(out, /\[store-engineer\] Bash go test/)
})
test('ended with PR', () => {
  const out = toPlain(buildRows(ended, NOW))
  assert.match(out, /30m00s · success · \S+pull\/7/)
  assert.match(out, /2\/2 verified/)
  assert.ok(buildRows(ended, NOW).length <= 30)
})
test('fmtDuration', () => { assert.equal(fmtDuration(3725000), '1h02m') })
