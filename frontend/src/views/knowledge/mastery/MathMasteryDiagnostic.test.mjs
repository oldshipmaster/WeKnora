import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./MathMasteryDiagnostic.vue', import.meta.url), 'utf8')
const treeSource = readFileSync(new URL('./MathMasteryTree.vue', import.meta.url), 'utf8')
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

test('is reachable from the production mastery tree and refreshes assessment state', () => {
  assert.match(treeSource, /import MathMasteryDiagnostic from '\.\/MathMasteryDiagnostic\.vue'/)
  assert.match(treeSource, /<MathMasteryDiagnostic/)
  assert.match(treeSource, /:node="selectedNode"/)
  assert.match(treeSource, /:sources="selectedSources"/)
  assert.match(treeSource, /:enabled="selectedExamReady"/)
  assert.match(treeSource, /@assessment="handleDiagnosticAssessment"/)
  assert.match(treeSource, /async function handleDiagnosticAssessment/)
  assert.doesNotMatch(treeSource, /explainDiagnostic/)
})

test('keeps the single learner placeholder explicit and replaceable', () => {
  assert.match(source, /profileId\?: string/)
  assert.match(source, /props\.profileId/)
})
