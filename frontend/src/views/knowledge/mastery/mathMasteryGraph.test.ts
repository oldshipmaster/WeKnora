import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildDependencyPath,
  collectBlockedChain,
  filterMasteryNodes,
  layoutMasteryGraph,
} from './mathMasteryGraph'

const nodes = [
  { id: 'g2', grade: 2, term: 1, domain: 'operations', title: '乘法', sort_order: 1, assessment: { state: 'untested' }, blocked_by: ['g1'] },
  { id: 'g1b', grade: 1, term: 2, domain: 'geometry', title: '图形', sort_order: 2, assessment: { state: 'mastered' }, blocked_by: [] },
  { id: 'g1', grade: 1, term: 1, domain: 'number', title: '数的认识', sort_order: 1, assessment: { state: 'weak' }, blocked_by: [] },
] as const

test('lays out grades left-to-right and terms in stable vertical order', () => {
  const layout = layoutMasteryGraph(nodes, { columnWidth: 240, rowHeight: 112 })

  assert.deepEqual(layout.map(({ id, x, y }) => ({ id, x, y })), [
    { id: 'g1', x: 0, y: 0 },
    { id: 'g1b', x: 0, y: 112 },
    { id: 'g2', x: 240, y: 0 },
  ])
})

test('builds a cubic dependency path between node edges', () => {
  assert.equal(
    buildDependencyPath({ x: 10, y: 20 }, { x: 250, y: 132 }, { width: 180, height: 80 }),
    'M 190 60 C 220 60, 220 172, 250 172',
  )
})

test('filters by grade, term, domain and mastery state together', () => {
  const result = filterMasteryNodes(nodes, { grade: 1, term: 2, domain: 'geometry', state: 'mastered' })
  assert.deepEqual(result.map(node => node.id), ['g1b'])
})

test('collects the complete weak prerequisite chain without duplicates', () => {
  const graphNodes = [
    ...nodes,
    { id: 'g3', grade: 3, term: 1, domain: 'operations', title: '多位数乘法', sort_order: 1, assessment: { state: 'developing' }, blocked_by: ['g2'] },
  ] as const
  const edges = [
    { from_node_id: 'g1', to_node_id: 'g2' },
    { from_node_id: 'g2', to_node_id: 'g3' },
    { from_node_id: 'g1', to_node_id: 'g3' },
  ]

  assert.deepEqual(collectBlockedChain('g3', graphNodes, edges), ['g1', 'g2'])
})
