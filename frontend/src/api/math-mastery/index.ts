import { get, post, put } from '@/utils/request'

export type MathMasteryState = 'untested' | 'weak' | 'developing' | 'mastered'
export type MathMaterialStatus = 'found' | 'missing' | 'uploaded' | 'processing' | 'ready' | 'failed'

export interface MathMasteryAssessment {
  state: MathMasteryState
  score: number
  confidence: number
  evidence_count: number
  reasons: string[]
}

export interface MathCurriculumNode {
  id: string
  parent_id?: string
  node_type: string
  grade: number
  term: number
  domain: string
  title: string
  summary?: string
  sort_order: number
  expected_question_count: number
}

export interface MathMasteryNode extends MathCurriculumNode {
  assessment: MathMasteryAssessment
  blocked_by: string[]
  question_count: number
}

export interface MathCurriculumEdge {
  id?: string
  from_node_id: string
  to_node_id: string
  relation_type: string
  strength: number
  rationale?: string
}

export interface MathSourceBinding {
  id: string
  target_id: string
  node_id?: string
  knowledge_id?: string
  source_type: 'textbook' | 'exam'
  status: MathMaterialStatus
  title: string
  edition: string
  school_year?: number
  season?: 'spring' | 'autumn'
  grade: number
  term: number
  content_hash?: string
  error_message?: string
}

export interface MathMasteryOverview {
  total_nodes: number
  untested_nodes: number
  weak_nodes: number
  developing_nodes: number
  mastered_nodes: number
  blocked_nodes: number
  coverage_rate: number
  mastery_rate: number
  textbooks_ready: number
  textbooks_total: number
  exams_ready: number
  exams_total: number
  diagnostic_questions: number
  nodes_with_questions: number
}

export interface MathMasteryTree {
  overview: MathMasteryOverview
  nodes: MathMasteryNode[]
  edges: MathCurriculumEdge[]
  sources: MathSourceBinding[]
}

export interface MathDiagnosticQuestion {
  id: string
  source_binding_id: string
  question_locator: string
  answer_locator?: string
  question_type: string
  difficulty: number
  extraction_confidence: number
  scoring_rule?: {
    prompt?: string
    answer_hint?: string
    [key: string]: unknown
  }
}

interface ApiResponse<T> {
  success: boolean
  data: T
}

export async function getMathMasteryTree(kbID: string): Promise<MathMasteryTree> {
  const response = await get<ApiResponse<MathMasteryTree>>(`/api/v1/knowledge-bases/${kbID}/math-mastery/tree`)
  return response.data
}

export function seedMathCurriculum(kbID: string, nodes: MathCurriculumNode[], edges: MathCurriculumEdge[]) {
  return post(`/api/v1/knowledge-bases/${kbID}/math-mastery/seed`, { nodes, edges })
}

export function upsertMathSources(kbID: string, sources: MathSourceBinding[]) {
  return put(`/api/v1/knowledge-bases/${kbID}/math-mastery/sources`, { sources })
}

export async function getMathDiagnosticQuestions(kbID: string, nodeID: string, limit = 5) {
  const response = await get<ApiResponse<MathDiagnosticQuestion[]>>(
    `/api/v1/knowledge-bases/${kbID}/math-mastery/questions`,
    { params: { node_id: nodeID, limit } },
  )
  return response.data
}

export async function startMathDiagnostic(kbID: string, profileID = 'local-child', scope: Record<string, unknown> = {}) {
  const response = await post<ApiResponse<{ id: string }>>(`/api/v1/knowledge-bases/${kbID}/math-mastery/attempts`, {
    profile_id: profileID,
    scope,
  })
  return response.data
}

export async function submitMathDiagnosticResponse(kbID: string, response: {
  attempt_id: string
  question_id: string
  node_id: string
  question_type: string
  correct: boolean
  duration_ms?: number
  self_confidence?: number
  error_type?: string
}) {
  const result = await post<ApiResponse<MathMasteryAssessment>>(`/api/v1/knowledge-bases/${kbID}/math-mastery/responses`, response)
  return result.data
}
