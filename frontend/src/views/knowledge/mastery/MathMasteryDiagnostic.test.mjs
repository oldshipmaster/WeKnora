import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./MathMasteryDiagnostic.vue', import.meta.url), 'utf8')
const api = readFileSync(new URL('../../../api/math-mastery/index.ts', import.meta.url), 'utf8')

test('runs an evidence-backed diagnostic from imported exam questions', () => {
  assert.match(api, /export async function getMathDiagnosticQuestions/)
  assert.match(api, /params:\s*\{\s*node_id:\s*nodeID,\s*limit\s*\}/)
  assert.match(source, /getMathDiagnosticQuestions/)
  assert.match(source, /startMathDiagnostic/)
  assert.match(source, /submitMathDiagnosticResponse/)
  assert.match(source, />答错</)
  assert.match(source, />答对</)
  assert.match(source, /question_locator/)
})
