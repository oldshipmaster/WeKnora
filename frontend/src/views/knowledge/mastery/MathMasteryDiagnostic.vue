<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  getMathDiagnosticQuestions,
  startMathDiagnostic,
  submitMathDiagnosticResponse,
  type MathDiagnosticQuestion,
  type MathMasteryAssessment,
  type MathMasteryNode,
  type MathSourceBinding,
} from '@/api/math-mastery'

const props = withDefaults(defineProps<{
  knowledgeBaseId: string
  node: MathMasteryNode
  sources?: MathSourceBinding[]
  profileId?: string
}>(), {
  sources: () => [],
  profileId: 'local-child',
})

const emit = defineEmits<{
  assessment: [assessment: MathMasteryAssessment, completed: boolean, nodeID: string]
}>()

const questions = ref<MathDiagnosticQuestion[]>([])
const questionIndex = ref(0)
const attemptID = ref('')
const activeNodeID = ref('')
const startedAt = ref(0)
const loading = ref(false)
const submitting = ref(false)
const active = ref(false)
const errorMessage = ref('')
const diagnosticGeneration = ref(0)

const currentQuestion = computed(() => questions.value[questionIndex.value])
const currentSource = computed(() => props.sources.find(source => source.id === currentQuestion.value?.source_binding_id))

function reset() {
  diagnosticGeneration.value += 1
  questions.value = []
  questionIndex.value = 0
  attemptID.value = ''
  activeNodeID.value = ''
  startedAt.value = 0
  loading.value = false
  submitting.value = false
  active.value = false
  errorMessage.value = ''
}

async function beginDiagnostic() {
  const generation = ++diagnosticGeneration.value
  const knowledgeBaseID = props.knowledgeBaseId
  const nodeID = props.node.id
  loading.value = true
  errorMessage.value = ''
  try {
    const importedQuestions = await getMathDiagnosticQuestions(knowledgeBaseID, nodeID, 5)
    if (generation !== diagnosticGeneration.value) return
    if (!importedQuestions.length) {
      errorMessage.value = '这个知识点还没有可追溯诊断题，请先完成对应试卷题目导入。'
      return
    }
    const attempt = await startMathDiagnostic(knowledgeBaseID, props.profileId, {
      node_id: nodeID,
      question_ids: importedQuestions.map(question => question.id),
    })
    if (generation !== diagnosticGeneration.value) return
    questions.value = importedQuestions
    questionIndex.value = 0
    attemptID.value = attempt.id
    activeNodeID.value = nodeID
    startedAt.value = Date.now()
    active.value = true
  } catch (error: any) {
    if (generation !== diagnosticGeneration.value) return
    errorMessage.value = error?.message || '诊断题加载失败，请稍后重试。'
  } finally {
    if (generation === diagnosticGeneration.value) loading.value = false
  }
}

async function recordAnswer(correct: boolean) {
  const question = currentQuestion.value
  const submittedAttemptID = attemptID.value
  const nodeID = activeNodeID.value
  if (!question || !submittedAttemptID || !nodeID) return
  const generation = diagnosticGeneration.value
  const knowledgeBaseID = props.knowledgeBaseId
  const submittedQuestionIndex = questionIndex.value
  const submittedQuestionCount = questions.value.length
  const submittedStartedAt = startedAt.value
  submitting.value = true
  errorMessage.value = ''
  try {
    const assessment = await submitMathDiagnosticResponse(knowledgeBaseID, {
      attempt_id: submittedAttemptID,
      question_id: question.id,
      node_id: nodeID,
      question_type: question.question_type,
      correct,
      duration_ms: Math.max(1, Date.now() - submittedStartedAt),
    })
    if (generation !== diagnosticGeneration.value) return
    const completed = submittedQuestionIndex >= submittedQuestionCount - 1
    emit('assessment', assessment, completed, nodeID)
    if (!completed) {
      questionIndex.value = submittedQuestionIndex + 1
      startedAt.value = Date.now()
      return
    }
    active.value = false
    MessagePlugin.success(`本轮诊断完成，已形成 ${assessment.evidence_count} 条作答证据。`)
  } catch (error: any) {
    if (generation !== diagnosticGeneration.value) return
    errorMessage.value = error?.message || '作答证据保存失败，请重试。'
  } finally {
    if (generation === diagnosticGeneration.value) submitting.value = false
  }
}

watch(() => [props.knowledgeBaseId, props.node.id], reset)
onBeforeUnmount(() => {
  diagnosticGeneration.value += 1
})
</script>

<template>
  <section class="diagnostic" aria-label="知识点诊断">
    <div v-if="active && currentQuestion" class="diagnostic-card" aria-live="polite">
      <div class="diagnostic-progress">
        <span>第 {{ questionIndex + 1 }} / {{ questions.length }} 题</span>
        <span>难度 {{ Math.round(currentQuestion.difficulty * 100) }}</span>
      </div>
      <strong>{{ currentQuestion.scoring_rule?.prompt || '请回看原试卷完成这道题。' }}</strong>
      <p class="diagnostic-locator">{{ currentQuestion.question_locator }}</p>
      <p v-if="currentSource" class="diagnostic-source">来源：{{ currentSource.title }}</p>
      <p class="diagnostic-instruction">
        请孩子先在纸上作答，再由家长核对原卷或答案并记录结果；本页面不进行自动判题。
      </p>
      <p v-if="currentQuestion.scoring_rule?.answer_hint" class="diagnostic-hint">
        核对提示：{{ currentQuestion.scoring_rule.answer_hint }}
      </p>
      <div class="diagnostic-actions">
        <t-button theme="danger" variant="outline" :loading="submitting" @click="recordAnswer(false)">家长记录：答错</t-button>
        <t-button theme="success" :loading="submitting" @click="recordAnswer(true)">家长记录：答对</t-button>
      </div>
    </div>

    <p v-if="errorMessage" class="diagnostic-error" role="alert">{{ errorMessage }}</p>
    <t-button v-if="!active" block theme="primary" :loading="loading" @click="beginDiagnostic">
      {{ questions.length ? '再测一轮' : '开始诊断' }}
    </t-button>
  </section>
</template>

<style scoped>
.diagnostic { display: grid; gap: 12px; }
.diagnostic-card {
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid rgba(119, 199, 164, 0.38);
  border-radius: 12px;
  background: #10241f;
}
.diagnostic-progress { display: flex; justify-content: space-between; color: #9bb0ae; font-size: 11px; }
.diagnostic-card strong { color: #eef5f3; font-size: 15px; line-height: 1.65; white-space: pre-wrap; }
.diagnostic-locator,
.diagnostic-source,
.diagnostic-instruction,
.diagnostic-hint { margin: 0; color: #9bb0ae; font-size: 12px; line-height: 1.5; }
.diagnostic-locator { color: #77c7a4; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.diagnostic-instruction { color: #d7e7e2; }
.diagnostic-hint { padding: 9px 10px; border-radius: 8px; background: rgba(255, 255, 255, 0.045); }
.diagnostic-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.diagnostic-error { margin: 0; color: #f1aaa2; font-size: 12px; line-height: 1.55; }
</style>
