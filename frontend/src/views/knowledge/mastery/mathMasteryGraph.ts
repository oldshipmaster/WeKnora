export type MasteryState = 'untested' | 'weak' | 'developing' | 'mastered'

export interface MasteryNodeLike {
  id: string
  grade: number
  term: number
  domain: string
  title: string
  sort_order: number
  assessment: { state: MasteryState | string }
  blocked_by: readonly string[]
}

export interface MasteryEdgeLike {
  from_node_id: string
  to_node_id: string
}

export interface MasteryFilters {
  grade?: number
  term?: number
  domain?: string
  state?: MasteryState | string
}

export interface PositionedMasteryNode<T extends MasteryNodeLike = MasteryNodeLike> {
  id: string
  x: number
  y: number
  node: T
}

export function layoutMasteryGraph<T extends MasteryNodeLike>(
  nodes: readonly T[],
  options: { columnWidth: number; rowHeight: number },
): PositionedMasteryNode<T>[] {
  const sorted = [...nodes].sort((left, right) =>
    left.grade - right.grade ||
    left.term - right.term ||
    left.sort_order - right.sort_order ||
    left.id.localeCompare(right.id),
  )
  const rowsByGrade = new Map<number, number>()
  return sorted.map(node => {
    const row = rowsByGrade.get(node.grade) ?? 0
    rowsByGrade.set(node.grade, row + 1)
    return {
      id: node.id,
      x: (node.grade - 1) * options.columnWidth,
      y: row * options.rowHeight,
      node,
    }
  })
}

export function buildDependencyPath(
  from: { x: number; y: number },
  to: { x: number; y: number },
  size: { width: number; height: number },
): string {
  const startX = from.x + size.width
  const startY = from.y + size.height / 2
  const endX = to.x
  const endY = to.y + size.height / 2
  const controlX = startX + (endX - startX) / 2
  return `M ${startX} ${startY} C ${controlX} ${startY}, ${controlX} ${endY}, ${endX} ${endY}`
}

export function filterMasteryNodes<T extends MasteryNodeLike>(nodes: readonly T[], filters: MasteryFilters): T[] {
  return nodes.filter(node =>
    (filters.grade === undefined || node.grade === filters.grade) &&
    (filters.term === undefined || node.term === filters.term) &&
    (!filters.domain || node.domain === filters.domain) &&
    (!filters.state || node.assessment.state === filters.state),
  )
}

export function collectBlockedChain<T extends MasteryNodeLike>(
  targetID: string,
  nodes: readonly T[],
  edges: readonly MasteryEdgeLike[],
): string[] {
  const nodeByID = new Map(nodes.map(node => [node.id, node]))
  const incoming = new Map<string, string[]>()
  for (const edge of edges) {
    incoming.set(edge.to_node_id, [...(incoming.get(edge.to_node_id) ?? []), edge.from_node_id])
  }

  const blocked = new Set<string>()
  const visit = (nodeID: string) => {
    for (const prerequisiteID of incoming.get(nodeID) ?? []) {
      const prerequisite = nodeByID.get(prerequisiteID)
      if (!prerequisite || prerequisite.assessment.state === 'mastered' || blocked.has(prerequisiteID)) continue
      blocked.add(prerequisiteID)
      visit(prerequisiteID)
    }
  }
  visit(targetID)

  return nodes
    .filter(node => blocked.has(node.id))
    .sort((left, right) => left.grade - right.grade || left.term - right.term || left.sort_order - right.sort_order)
    .map(node => node.id)
}
