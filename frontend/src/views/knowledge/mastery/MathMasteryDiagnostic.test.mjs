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
  assert.match(source, /recordAnswer\(false\)/)
  assert.match(source, /recordAnswer\(true\)/)
  assert.match(source, /question_locator/)
})

test('is reachable from the production mastery tree and refreshes assessment state', () => {
  assert.match(treeSource, /import MathMasteryDiagnostic from '\.\/MathMasteryDiagnostic\.vue'/)
  assert.match(treeSource, /<MathMasteryDiagnostic/)
  assert.match(treeSource, /:node="selectedNode"/)
  assert.match(treeSource, /:sources="selectedSources"/)
  assert.match(treeSource, /@assessment="handleDiagnosticAssessment"/)
  assert.match(treeSource, /async function handleDiagnosticAssessment/)
  assert.doesNotMatch(treeSource, /explainDiagnostic/)
})

test('keeps the diagnostic mounted until the complete question set is answered', () => {
  assert.match(source, /const completed = submittedQuestionIndex >= submittedQuestionCount - 1/)
  assert.match(source, /emit\('assessment', assessment, completed, nodeID\)/)
  assert.match(treeSource, /handleDiagnosticAssessment\(assessment: MathMasteryAssessment, completed: boolean, nodeID: string\)/)
  assert.match(treeSource, /if \(!completed\) return\s+await loadTree\(\)/)
})

test('ignores stale diagnostic requests after switching knowledge nodes', () => {
  assert.match(source, /const diagnosticGeneration = ref\(0\)/)
  assert.match(source, /const generation = \+\+diagnosticGeneration\.value/)
  assert.match(source, /const nodeID = props\.node\.id/)
  assert.match(source, /if \(generation !== diagnosticGeneration\.value\) return/)
  assert.match(source, /const submittedQuestionIndex = questionIndex\.value/)
  assert.match(source, /const submittedQuestionCount = questions\.value\.length/)
  assert.match(source, /emit\('assessment', assessment, completed, nodeID\)/)
  assert.match(treeSource, /handleDiagnosticAssessment\(assessment: MathMasteryAssessment, completed: boolean, nodeID: string\)/)
  assert.match(treeSource, /nodes\.value\.find\(node => node\.id === nodeID\)/)
})

test('invalidates pending requests when the diagnostic is unmounted or changes knowledge base', () => {
  assert.match(source, /import \{ computed, onBeforeUnmount, ref, watch \} from 'vue'/)
  assert.match(source, /watch\(\(\) => \[props\.knowledgeBaseId, props\.node\.id\], reset\)/)
  assert.match(source, /onBeforeUnmount\(\(\) => \{\s*diagnosticGeneration\.value \+= 1\s*\}\)/)
})

test('uses imported questions as the readiness authority instead of source parsing status', () => {
  assert.doesNotMatch(treeSource, /selectedExamReady/)
  assert.doesNotMatch(treeSource, /:enabled=/)
  assert.doesNotMatch(source, /enabled\?: boolean/)
  assert.doesNotMatch(source, /:disabled="!enabled"/)
  assert.match(source, /const importedQuestions = await getMathDiagnosticQuestions/)
  assert.match(source, /if \(!importedQuestions\.length\)/)
})

test('exposes imported question readiness on the tree without conflating it with parse status', () => {
  assert.match(api, /question_count:\s*number/)
  assert.match(api, /diagnostic_questions:\s*number/)
  assert.match(api, /nodes_with_questions:\s*number/)
  assert.match(treeSource, /selectedNode\.question_count/)
  assert.match(treeSource, /label:\s*'诊断题库'.*diagnostic_questions/s)
  assert.match(treeSource, /nodes_with_questions.*知识点/s)
  assert.match(treeSource, /资料优化状态不影响开始作答/)
})

test('makes the paper-and-parent verification workflow explicit', () => {
  assert.match(source, /请孩子先在纸上作答/)
  assert.match(source, /家长核对/)
  assert.match(source, /不进行自动判题/)
  assert.match(source, />家长记录：答错</)
  assert.match(source, />家长记录：答对</)
})

test('keeps the single learner placeholder explicit and replaceable', () => {
  assert.match(source, /profileId\?: string/)
  assert.match(source, /props\.profileId/)
})
