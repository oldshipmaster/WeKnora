import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./KnowledgeBase.vue', import.meta.url), 'utf8')

test('renders the mastery navigation as live breadcrumb content', () => {
  const breadcrumb = source.match(
    /<h2 class="document-breadcrumb">([\s\S]*?)<\/h2>/,
  )?.[1]

  assert.ok(breadcrumb, 'expected to find the knowledge-base breadcrumb')
  assert.doesNotMatch(
    breadcrumb,
    /<template>\s*<span[^>]+activeKbTab === 'documents'/,
    'an unconditional native template hides all breadcrumb tabs at runtime',
  )
  assert.match(breadcrumb, /activeKbTab === 'mastery'/)
})
