<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  getMathMasteryTree,
  type MathMasteryAssessment,
  type MathMasteryNode,
  type MathMasteryState,
  type MathMasteryTree,
  type MathSourceBinding,
} from '@/api/math-mastery'
import MathMasteryDiagnostic from './MathMasteryDiagnostic.vue'
import { buildDependencyPath, collectBlockedChain, filterMasteryNodes, layoutMasteryGraph } from './mathMasteryGraph'

const props = defineProps<{ knowledgeBaseId: string }>()

const NODE_WIDTH = 184
const NODE_HEIGHT = 82
const COLUMN_WIDTH = 244
const ROW_HEIGHT = 108
const CANVAS_PADDING = 42

const loading = ref(true)
const errorMessage = ref('')
const tree = ref<MathMasteryTree | null>(null)
const selectedNodeID = ref('')
const gradeFilter = ref<number | undefined>()
const termFilter = ref<number | undefined>()
const domainFilter = ref('')
const stateFilter = ref('')
const blockedOnly = ref(false)

const domainLabels: Record<string, string> = {
  number: '数的认识',
  operations: '运算',
  algebra: '数量关系',
  geometry: '图形与几何',
  measurement: '测量',
  statistics: '统计与概率',
  application: '综合应用',
}

const stateLabels: Record<MathMasteryState, string> = {
  untested: '未测',
  weak: '薄弱',
  developing: '形成中',
  mastered: '稳固',
}

const sourceStatusLabels: Record<string, string> = {
  found: '已发现',
  missing: '缺失',
  uploaded: '已上传',
  processing: '处理中',
  ready: '完成',
  failed: '失败',
}

const nodes = computed(() => tree.value?.nodes ?? [])
const edges = computed(() => tree.value?.edges ?? [])
const sources = computed(() => tree.value?.sources ?? [])

const domains = computed(() => [...new Set(nodes.value.map(node => node.domain))])

const filteredNodes = computed(() => {
  let result = filterMasteryNodes(nodes.value, {
    grade: gradeFilter.value,
    term: termFilter.value,
    domain: domainFilter.value || undefined,
    state: stateFilter.value || undefined,
  })
  if (blockedOnly.value) result = result.filter(node => node.blocked_by.length > 0)
  return result
})

const positionedNodes = computed(() => layoutMasteryGraph(filteredNodes.value, {
  columnWidth: COLUMN_WIDTH,
  rowHeight: ROW_HEIGHT,
}).map(item => ({ ...item, x: item.x + CANVAS_PADDING, y: item.y + CANVAS_PADDING + 28 })))

const positionByID = computed(() => new Map(positionedNodes.value.map(item => [item.id, item])))
const visibleEdges = computed(() => edges.value.filter(edge => positionByID.value.has(edge.from_node_id) && positionByID.value.has(edge.to_node_id)))

const canvasSize = computed(() => {
  const maxX = Math.max(...positionedNodes.value.map(node => node.x), 0)
  const maxY = Math.max(...positionedNodes.value.map(node => node.y), 0)
  return {
    width: Math.max(960, maxX + NODE_WIDTH + CANVAS_PADDING),
    height: Math.max(520, maxY + NODE_HEIGHT + CANVAS_PADDING),
  }
})

const selectedNode = computed(() => nodes.value.find(node => node.id === selectedNodeID.value))
const selectedBlockedChain = computed(() => selectedNode.value
  ? collectBlockedChain(selectedNode.value.id, nodes.value, edges.value)
    .map(id => nodes.value.find(node => node.id === id))
    .filter((node): node is MathMasteryNode => !!node)
  : [])
const selectedSources = computed(() => selectedNode.value
  ? sources.value.filter(source => source.node_id === selectedNode.value?.id || (!source.node_id && source.grade === selectedNode.value?.grade && source.term === selectedNode.value?.term))
  : [])
const selectedExamReady = computed(() => selectedSources.value.some(source => source.source_type === 'exam' && source.status === 'ready'))

const textbookSources = computed(() => sources.value.filter(source => source.source_type === 'textbook'))

const metricItems = computed(() => {
  const overview = tree.value?.overview
  return [
    { label: '稳固知识点', value: overview?.mastered_nodes ?? 0, detail: `共 ${overview?.total_nodes ?? 0} 个` },
    { label: '关键薄弱点', value: overview?.weak_nodes ?? 0, detail: `${overview?.blocked_nodes ?? 0} 个下游受阻` },
    { label: '待验证', value: overview?.untested_nodes ?? 0, detail: `${overview?.developing_nodes ?? 0} 个形成中` },
    { label: '教材素材', value: `${overview?.textbooks_ready ?? 0}/${overview?.textbooks_total || 12}`, detail: '人教版一至六年级' },
    { label: '2026 试卷', value: `${overview?.exams_ready ?? 0}/${overview?.exams_total || 12}`, detail: '缺失时不使用旧版替代' },
  ]
})

function edgePath(fromID: string, toID: string) {
  const from = positionByID.value.get(fromID)
  const to = positionByID.value.get(toID)
  if (!from || !to) return ''
  return buildDependencyPath(from, to, { width: NODE_WIDTH, height: NODE_HEIGHT })
}

function selectNode(node: MathMasteryNode) {
  selectedNodeID.value = node.id
}

function resetFilters() {
  gradeFilter.value = undefined
  termFilter.value = undefined
  domainFilter.value = ''
  stateFilter.value = ''
  blockedOnly.value = false
}

async function loadTree() {
  loading.value = true
  errorMessage.value = ''
  try {
    tree.value = await getMathMasteryTree(props.knowledgeBaseId)
  } catch (error: any) {
    errorMessage.value = error?.message || '掌握树加载失败，请检查后端服务与迁移状态。'
  } finally {
    loading.value = false
  }
}

async function handleDiagnosticAssessment(assessment: MathMasteryAssessment) {
  if (selectedNode.value) selectedNode.value.assessment = assessment
  await loadTree()
}

function statusClass(source: MathSourceBinding) {
  return `source-status source-status--${source.status}`
}

onMounted(loadTree)
</script>

<template>
  <section class="mastery-shell" aria-labelledby="mastery-title">
    <header class="mastery-header">
      <div>
        <p class="mastery-kicker">人教版小学数学</p>
        <h2 id="mastery-title">知识体系掌握树</h2>
        <p class="mastery-intro">从真实作答证据判断孩子是否形成完整知识链。每个结论都能追溯到教材、试卷与诊断记录。</p>
      </div>
      <t-button theme="default" variant="outline" :loading="loading" @click="loadTree">刷新状态</t-button>
    </header>

    <div v-if="loading" class="mastery-loading" aria-live="polite">
      <t-skeleton animation="gradient" :row-col="[[1, 1, 1], [1], [1], [1]]" />
    </div>

    <div v-else-if="errorMessage" class="mastery-error" role="alert">
      <strong>暂时无法读取掌握树</strong>
      <p>{{ errorMessage }}</p>
      <t-button theme="primary" @click="loadTree">重新加载</t-button>
    </div>

    <template v-else-if="tree">
      <div class="mastery-metrics" aria-label="掌握概览">
        <article v-for="(metric, index) in metricItems" :key="metric.label" :class="['metric', { 'metric--primary': index === 0 }]">
          <span>{{ metric.label }}</span>
          <strong>{{ metric.value }}</strong>
          <small>{{ metric.detail }}</small>
        </article>
      </div>

      <div class="mastery-filterbar" aria-label="知识树筛选">
        <t-select v-model="gradeFilter" clearable placeholder="全部年级" class="filter-select">
          <t-option v-for="grade in 6" :key="grade" :value="grade" :label="`${grade} 年级`" />
        </t-select>
        <t-select v-model="termFilter" clearable placeholder="上下册" class="filter-select">
          <t-option :value="1" label="上册" />
          <t-option :value="2" label="下册" />
        </t-select>
        <t-select v-model="domainFilter" clearable placeholder="全部领域" class="filter-select filter-select--wide">
          <t-option v-for="domain in domains" :key="domain" :value="domain" :label="domainLabels[domain] || domain" />
        </t-select>
        <t-select v-model="stateFilter" clearable placeholder="掌握状态" class="filter-select">
          <t-option v-for="(label, state) in stateLabels" :key="state" :value="state" :label="label" />
        </t-select>
        <t-checkbox v-model="blockedOnly">仅看阻塞链</t-checkbox>
        <button type="button" class="filter-reset" @click="resetFilters">清除筛选</button>
      </div>

      <div v-if="!nodes.length" class="mastery-empty">
        <strong>课程树尚未初始化</strong>
        <p>先运行素材导入命令，它会同步人教版课程节点与 12 册教材状态。</p>
      </div>

      <div v-else class="mastery-workspace">
        <div class="tree-viewport" tabindex="0" aria-label="可横向滚动的六年级知识树">
          <div class="tree-canvas" :style="{ width: `${canvasSize.width}px`, height: `${canvasSize.height}px` }">
            <div v-for="grade in 6" :key="grade" class="grade-label" :style="{ left: `${CANVAS_PADDING + (grade - 1) * COLUMN_WIDTH}px` }">
              <span>{{ grade }} 年级</span>
            </div>
            <svg class="dependency-layer" :width="canvasSize.width" :height="canvasSize.height" aria-hidden="true">
              <path v-for="edge in visibleEdges" :key="edge.id || `${edge.from_node_id}-${edge.to_node_id}`"
                :d="edgePath(edge.from_node_id, edge.to_node_id)" />
            </svg>
            <button v-for="item in positionedNodes" :key="item.id" type="button"
              :class="['mastery-node', `mastery-node--${item.node.assessment.state}`, `mastery-node--${item.node.domain}`, { selected: selectedNodeID === item.id }]"
              :style="{ transform: `translate(${item.x}px, ${item.y}px)`, width: `${NODE_WIDTH}px`, height: `${NODE_HEIGHT}px` }"
              :aria-label="`${item.node.title}，${stateLabels[item.node.assessment.state]}`"
              @click="selectNode(item.node)">
              <span class="node-domain">{{ domainLabels[item.node.domain] || item.node.domain }}</span>
              <strong>{{ item.node.title }}</strong>
              <span class="node-state">{{ stateLabels[item.node.assessment.state] }}</span>
              <span v-if="item.node.blocked_by.length" class="node-blocked">{{ item.node.blocked_by.length }} 个前置待补</span>
            </button>
          </div>
        </div>

        <aside class="mastery-drawer" :class="{ 'mastery-drawer--empty': !selectedNode }">
          <template v-if="selectedNode">
            <div class="drawer-heading">
              <div>
                <span>{{ selectedNode.grade }} 年级 {{ selectedNode.term === 1 ? '上册' : '下册' }}</span>
                <h3>{{ selectedNode.title }}</h3>
              </div>
              <button type="button" class="drawer-close" aria-label="关闭知识点详情" @click="selectedNodeID = ''">关闭</button>
            </div>
            <div :class="['drawer-state', `drawer-state--${selectedNode.assessment.state}`]">
              <strong>{{ stateLabels[selectedNode.assessment.state] }}</strong>
              <span>得分 {{ Math.round(selectedNode.assessment.score * 100) }}%</span>
              <span>置信度 {{ Math.round(selectedNode.assessment.confidence * 100) }}%</span>
              <span>{{ selectedNode.assessment.evidence_count }} 条证据</span>
            </div>
            <section class="drawer-section">
              <h4>判断依据</h4>
              <p v-for="reason in selectedNode.assessment.reasons" :key="reason">{{ reason }}</p>
            </section>
            <section v-if="selectedBlockedChain.length" class="drawer-section">
              <h4>先修阻塞链</h4>
              <button v-for="node in selectedBlockedChain" :key="node.id" type="button" class="blocked-link" @click="selectNode(node)">
                {{ node.grade }} 年级 / {{ node.title }} / {{ stateLabels[node.assessment.state] }}
              </button>
            </section>
            <section class="drawer-section">
              <h4>素材证据</h4>
              <p v-if="!selectedSources.length" class="muted">尚未绑定到教材或试卷定位。</p>
              <div v-for="source in selectedSources" :key="source.id" class="source-row">
                <span>{{ source.title }}</span>
                <em :class="statusClass(source)">{{ sourceStatusLabels[source.status] || source.status }}</em>
              </div>
            </section>
            <MathMasteryDiagnostic
              :knowledge-base-id="props.knowledgeBaseId"
              :node="selectedNode"
              :sources="selectedSources"
              :enabled="selectedExamReady"
              @assessment="handleDiagnosticAssessment"
            />
          </template>
          <template v-else>
            <strong>选择一个知识点</strong>
            <p>查看掌握依据、先修阻塞链和对应素材。状态不能手工修改。</p>
          </template>
        </aside>
      </div>

      <section class="material-panel" aria-labelledby="material-title">
        <div class="material-heading">
          <div>
            <h3 id="material-title">素材完整度</h3>
            <p>教材与指定年份试卷分别核验，旧版素材不会被自动替代。</p>
          </div>
          <span>{{ textbookSources.filter(source => source.status === 'ready').length }}/{{ textbookSources.length || 12 }} 册教材完成</span>
        </div>
        <div v-if="!sources.length" class="material-empty">素材 manifest 尚未同步到知识库。</div>
        <div v-else class="material-grid">
          <article v-for="source in sources" :key="source.id" class="material-item">
            <span>{{ source.grade }} 年级{{ source.term === 1 ? '上册' : '下册' }}</span>
            <strong>{{ source.source_type === 'textbook' ? '人教版数学教材' : source.title }}</strong>
            <em :class="statusClass(source)">{{ sourceStatusLabels[source.status] || source.status }}</em>
            <small v-if="source.error_message">{{ source.error_message }}</small>
          </article>
        </div>
      </section>
    </template>
  </section>
</template>

<style scoped>
.mastery-shell {
  --surface: #0d171b;
  --surface-raised: #132126;
  --surface-soft: #182a30;
  --line: rgba(185, 217, 218, 0.16);
  --text: #eef5f3;
  --muted: #9bb0ae;
  --accent: #77c7a4;
  min-height: 100%;
  padding: 28px;
  color: var(--text);
  background: var(--surface);
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
}

.mastery-header,
.material-heading,
.drawer-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.mastery-kicker {
  margin: 0 0 8px;
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.16em;
}

.mastery-header h2,
.material-heading h3,
.drawer-heading h3 {
  margin: 0;
  color: var(--text);
}

.mastery-header h2 { font-size: 28px; letter-spacing: -0.02em; }
.mastery-intro { max-width: 680px; margin: 10px 0 0; color: var(--muted); line-height: 1.7; }

.mastery-loading,
.mastery-error,
.mastery-empty,
.material-empty {
  margin-top: 24px;
  padding: 32px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--surface-raised);
}

.mastery-error p,
.mastery-empty p,
.mastery-drawer p { color: var(--muted); line-height: 1.6; }

.mastery-metrics {
  display: grid;
  grid-template-columns: 1.35fr repeat(4, 1fr);
  gap: 10px;
  margin-top: 26px;
}

.metric {
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--surface-raised);
}

.metric--primary { background: #173229; border-color: rgba(119, 199, 164, 0.4); }
.metric span, .metric small { display: block; color: var(--muted); }
.metric strong { display: block; margin: 7px 0 4px; font: 700 26px/1.1 ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--text); }
.metric small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.mastery-filterbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--surface-raised);
}

.filter-select { width: 126px; }
.filter-select--wide { width: 150px; }
.filter-reset, .drawer-close, .blocked-link {
  border: 0;
  color: var(--accent);
  background: transparent;
  cursor: pointer;
}
.filter-reset { margin-left: auto; }
.filter-reset:focus-visible, .drawer-close:focus-visible, .blocked-link:focus-visible, .mastery-node:focus-visible {
  outline: 2px solid #b8e7d2;
  outline-offset: 3px;
}

.mastery-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 310px;
  gap: 14px;
  margin-top: 14px;
}

.tree-viewport {
  min-width: 0;
  overflow: auto;
  border: 1px solid var(--line);
  border-radius: 14px;
  background-color: #0a1418;
  background-image: radial-gradient(circle, rgba(143, 181, 179, 0.18) 1px, transparent 1px);
  background-size: 22px 22px;
  scrollbar-color: #39585b #101e23;
}

.tree-canvas { position: relative; }
.grade-label { position: absolute; top: 16px; width: 184px; color: #abc0bd; font: 700 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.grade-label span { display: inline-block; padding-bottom: 7px; border-bottom: 2px solid var(--accent); }
.dependency-layer { position: absolute; inset: 0; overflow: visible; pointer-events: none; }
.dependency-layer path { fill: none; stroke: rgba(134, 181, 173, 0.42); stroke-width: 1.5; vector-effect: non-scaling-stroke; }

.mastery-node {
  position: absolute;
  inset: 0 auto auto 0;
  display: grid;
  grid-template-columns: 1fr auto;
  align-content: center;
  gap: 5px 8px;
  padding: 12px 14px;
  border: 1px solid rgba(159, 190, 188, 0.28);
  border-left: 4px solid var(--node-color, #799b98);
  border-radius: 12px;
  color: var(--text);
  text-align: left;
  background: #132329;
  box-shadow: 0 10px 28px rgba(3, 9, 11, 0.22);
  cursor: pointer;
  transition: transform 140ms ease, border-color 140ms ease, background-color 140ms ease;
}

.mastery-node:hover { background: #192e35; border-color: rgba(210, 234, 228, 0.5); }
.mastery-node:active { margin-top: 1px; }
.mastery-node.selected { border-color: var(--accent); box-shadow: inset 0 0 0 1px var(--accent); }
.mastery-node--number { --node-color: #65bba8; }
.mastery-node--operations { --node-color: #dfaa62; }
.mastery-node--algebra { --node-color: #b290c5; }
.mastery-node--geometry { --node-color: #6da4ce; }
.mastery-node--measurement { --node-color: #ce7e78; }
.mastery-node--statistics { --node-color: #9fb766; }
.mastery-node--application { --node-color: #bc8f6c; }
.mastery-node--untested { opacity: 0.72; }
.mastery-node--weak { background: #34201f; }
.mastery-node--developing { background: #302a1b; }
.mastery-node--mastered { background: #173329; }
.node-domain { color: var(--node-color); font-size: 11px; font-weight: 700; }
.mastery-node strong { grid-column: 1 / -1; overflow: hidden; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.node-state, .node-blocked { color: var(--muted); font-size: 10px; }
.node-blocked { justify-self: end; color: #e8a69f; }

.mastery-drawer {
  min-height: 520px;
  padding: 20px;
  overflow: auto;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--surface-raised);
}
.mastery-drawer--empty { display: grid; align-content: center; text-align: center; }
.drawer-heading span { color: var(--muted); font-size: 12px; }
.drawer-heading h3 { margin-top: 6px; font-size: 20px; }
.drawer-close { padding: 4px 0; }
.drawer-state { display: flex; flex-wrap: wrap; gap: 8px 12px; margin-top: 18px; padding: 13px; border-radius: 12px; background: var(--surface-soft); }
.drawer-state strong { width: 100%; }
.drawer-state span { color: var(--muted); font-size: 12px; }
.drawer-state--weak strong { color: #f1aaa2; }
.drawer-state--developing strong { color: #ecc47a; }
.drawer-state--mastered strong { color: #8ad9b5; }
.drawer-section { margin: 20px 0; }
.drawer-section h4 { margin: 0 0 9px; font-size: 13px; }
.drawer-section p { margin: 6px 0; font-size: 13px; }
.blocked-link { display: block; width: 100%; padding: 7px 0; color: #e3beb9; text-align: left; }
.source-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 7px 0; color: var(--muted); font-size: 12px; }

.material-panel { margin-top: 14px; padding: 20px; border: 1px solid var(--line); border-radius: 14px; background: var(--surface-raised); }
.material-heading p { margin: 6px 0 0; color: var(--muted); }
.material-heading > span { color: var(--accent); font: 700 13px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace; }
.material-empty { margin-top: 16px; padding: 20px; color: var(--muted); background: var(--surface-soft); }
.material-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 9px; margin-top: 16px; }
.material-item { display: grid; grid-template-columns: 1fr auto; gap: 5px 10px; min-width: 0; padding: 12px; border-radius: 12px; background: var(--surface-soft); }
.material-item > span, .material-item > small { grid-column: 1 / -1; color: var(--muted); font-size: 11px; }
.material-item strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.material-item em, .source-status { align-self: center; font-size: 11px; font-style: normal; }
.source-status--ready { color: #8ad9b5; }
.source-status--missing, .source-status--failed { color: #f1aaa2; }
.source-status--processing, .source-status--uploaded { color: #ecc47a; }
.source-status--found { color: #9fc5d8; }
.muted { color: var(--muted); }

@media (max-width: 1100px) {
  .mastery-metrics { grid-template-columns: repeat(3, 1fr); }
  .mastery-workspace { grid-template-columns: 1fr; }
  .mastery-drawer { min-height: auto; }
  .material-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 767px) {
  .mastery-shell { padding: 16px; }
  .mastery-header { align-items: stretch; flex-direction: column; }
  .mastery-metrics { grid-template-columns: repeat(2, 1fr); }
  .metric--primary { grid-column: 1 / -1; }
  .mastery-filterbar { align-items: stretch; flex-direction: column; }
  .filter-select, .filter-select--wide { width: 100%; }
  .filter-reset { margin-left: 0; padding: 8px 0; text-align: left; }
  .material-heading { align-items: flex-start; flex-direction: column; }
  .material-grid { grid-template-columns: 1fr; }
}

@media (prefers-reduced-motion: reduce) {
  .mastery-node { transition: none; }
}
</style>
