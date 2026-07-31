#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

docker() {
  local joined=" $* "
  if [[ "${joined}" == *" WeKnora-postgres-dev "* && "${joined}" == *" psql "* ]]; then
    if [[ "${WEKNORA_TEST_EMPTY_PROJECTION:-false}" == "true" ]]; then
      printf '%s\n' '{"tenant_id":10000,"knowledge_base_id":"kb-test","node_ids":[],"edge_ids":[],"nodes":[],"edges":[]}'
      return 0
    fi
    printf '%s\n' '{"tenant_id":10000,"knowledge_base_id":"kb-test","node_ids":["node-a","node-b"],"edge_ids":["edge-a-b"],"nodes":[{"id":"node-a","parent_id":"","node_type":"unit","grade":1,"term":1,"domain":"number","title":"A","summary":"","sort_order":1,"expected_question_count":3,"metadata":{"source":"test"}},{"id":"node-b","parent_id":"","node_type":"unit","grade":1,"term":1,"domain":"number","title":"B","summary":"","sort_order":2,"expected_question_count":3,"metadata":{}}],"edges":[{"id":"edge-a-b","from_node_id":"node-a","to_node_id":"node-b","relation_type":"prerequisite","strength":1,"rationale":""}]}'
    return 0
  fi

  if [[ "${joined}" == *" WeKnora-neo4j-dev "* && "${joined}" == *" cypher-shell "* ]]; then
    [[ "${joined}" != *'test-only-password'* ]]
    local cypher="${*: -1}"
    [[ "${cypher}" == *'MERGE (node:MathCurriculum'* ]]
    [[ "${cypher}" == *'MERGE (source)-[relationship:CURRICULUM_EDGE'* ]]
    [[ "${cypher}" == *'DELETE relationship'* ]]
    [[ "${cypher}" != *'DETACH DELETE'* ]]
    [[ "${cypher}" == *'CALL apoc.util.validate('* ]]
    [[ "${cypher}" == *'node.metadata = apoc.convert.toJson(row.metadata)'* ]]
    [[ "${cypher}" == *'projected_nodes'* ]]
    printf '%s\n' 'nodes_upserted, relationships_replaced, edges_upserted, stale_nodes_deleted, projected_nodes, projected_relationships'
    printf '%s\n' '2, 0, 1, 0, 2, 1'
    return 0
  fi

  printf 'unexpected docker invocation\n' >&2
  return 64
}
export -f docker

output="$({
  WEKNORA_ENV_FILE=/dev/null \
  WEKNORA_MATH_KB_ID="kb-test" \
  WEKNORA_MATH_TENANT_ID=10000 \
  WEKNORA_POSTGRES_CONTAINER=WeKnora-postgres-dev \
  WEKNORA_NEO4J_CONTAINER=WeKnora-neo4j-dev \
  NEO4J_USERNAME=neo4j \
  NEO4J_PASSWORD=test-only-password \
  "${repo_root}/scripts/sync_math_mastery_graph.sh"
} 2>&1)"

[[ "${output}" == *'2, 0, 1, 0, 2, 1'* ]]

if invalid_output="$({
  WEKNORA_ENV_FILE=/dev/null \
  WEKNORA_MATH_KB_ID='invalid kb id' \
  NEO4J_USERNAME=neo4j \
  NEO4J_PASSWORD=test-only-password \
  "${repo_root}/scripts/sync_math_mastery_graph.sh"
} 2>&1)"; then
  printf 'invalid knowledge-base id unexpectedly succeeded\n' >&2
  exit 1
fi
[[ "${invalid_output}" == *'must be a safe knowledge-base id'* ]]

if empty_output="$({
  WEKNORA_ENV_FILE=/dev/null \
  WEKNORA_MATH_KB_ID=kb-test \
  WEKNORA_MATH_TENANT_ID=10000 \
  WEKNORA_POSTGRES_CONTAINER=WeKnora-postgres-dev \
  WEKNORA_NEO4J_CONTAINER=WeKnora-neo4j-dev \
  WEKNORA_TEST_EMPTY_PROJECTION=true \
  NEO4J_USERNAME=neo4j \
  NEO4J_PASSWORD=test-only-password \
  "${repo_root}/scripts/sync_math_mastery_graph.sh"
} 2>&1)"; then
  printf 'empty projection unexpectedly succeeded\n' >&2
  exit 1
fi
[[ "${empty_output}" == *'invalid or empty math curriculum projection'* ]]

printf 'sync_math_mastery_graph_test: PASS\n'
