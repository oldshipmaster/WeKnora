<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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
  enabled?: boolean
}>(), {
  sources: () => [],
  enabled: true,
})

const emit = defineEmits<{
  assessment: [assessment: MathMasteryAssessment]
}>()

const questions = ref<MathDiagnosticQuestion[]>([])
const questionIndex = ref(0)
const attemptID = ref('')
const startedAt = ref(0)
const loading = ref(false)
const submitting = ref(false)
const active = ref(false)
const errorMessage = ref('')

const currentQuestion = computed(() => questions.value[questionIndex.value])
const currentSource = computed(() => props.sources.find(source => source.id === currentQuestion.value?.source_binding_id))

function reset() {
  questions.value = []
  questionIndex.value = 0
  attemptID.value = ''
  startedAt.value = 0
  active.value = false
  errorMessage.value = ''
}

async function beginDiagnostic() {
  loading.value = true
  errorMessage.value = ''
  try {
    const importedQuestions = await getMathDiagnosticQuestions(props.knowledgeBaseId, props.node.id, 5)
    if (!importedQuestions.length) {
      errorMessage.value = '这个知识点还没有可追溯诊断题，请先完成对应试卷题目导入。'
      return
    }
    const attempt = await startMathDiagnostic(props.knowledgeBaseId, 'local-child', {
      node_id: props.node.id,
      question_ids: importedQuestions.map(question => question.id),
    })
    questions.value = importedQuestions
    questionIndex.value = 0
    attemptID.value = attempt.id
    startedAt.value = Date.now()
    active.value = true
  } catch (error: any) {
    errorMessage.value = error?.message || '诊断题加载失败，请稍后重试。'
  } finally {
    loading.value = false
  }
}

async function recordAnswer(correct: boolean) {
  if (!currentQuestion.value || !attemptID.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const assessment = await submitMathDiagnosticResponse(props.knowledgeBaseId, {
      attempt_id: attemptID.value,
      question_id: currentQuestion.value.id,
      node_id: props.node.id,
      question_type: currentQuestion.value.question_type,
      correct,
      duration_ms: Math.max(1, Date.now() - startedAt.value),
    })
    emit('assessment', assessment)
    if (questionIndex.value < questions.value.length - 1) {
      questionIndex.value += 1
      startedAt.value = Date.now()
      return
    }
    active.value = false
    MessagePlugin.success(`本轮诊断完成，已形成 ${assessment.evidence_count} 条作答证据。`)
  } catch (error: any) {
    errorMessage.value = error?.message || '作答证据保存失败，请重试。'
  } finally {
    submitting.value = false
  }
}

watch(() => props.node.id, reset)
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
      <p v-if="currentQuestion.scoring_rule?.answer_hint" class="diagnostic-hint">
        核对提示：{{ currentQuestion.scoring_rule.answer_hint }}
      </p>
      <div class="diagnostic-actions">
        <t-button theme="danger" variant="outline" :loading="submitting" @click="recordAnswer(false)">答错</t-button>
        <t-button theme="success" :loading="submitting" @click="recordAnswer(true)">答对</t-button>
      </div>
    </div>

    <p v-if="errorMessage" class="diagnostic-error" role="alert">{{ errorMessage }}</p>
    <t-button v-if="!active" block theme="primary" :disabled="!enabled" :loading="loading" @click="beginDiagnostic">
      {{ enabled ? (questions.length ? '再测一轮' : '开始诊断') : '诊断题库待补齐' }}
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
.diagnostic-hint { margin: 0; color: #9bb0ae; font-size: 12px; line-height: 1.5; }
.diagnostic-locator { color: #77c7a4; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.diagnostic-hint { padding: 9px 10px; border-radius: 8px; background: rgba(255, 255, 255, 0.045); }
.diagnostic-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.diagnostic-error { margin: 0; color: #f1aaa2; font-size: 12px; line-height: 1.55; }
</style>
