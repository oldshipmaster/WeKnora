#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="${WEKNORA_ENV_FILE:-${repo_root}/.env}"
if [[ -f "${env_file}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${env_file}"
  set +a
fi

: "${NEO4J_USERNAME:?NEO4J_USERNAME is required}"
: "${NEO4J_PASSWORD:?NEO4J_PASSWORD is required}"
export NEO4J_USERNAME NEO4J_PASSWORD

tenant_id=999001
kb_id="math-graph-sync-it-$$"
other_kb_id="${kb_id}-other"
postgres_container="${WEKNORA_POSTGRES_CONTAINER:-WeKnora-postgres-dev}"
neo4j_container="${WEKNORA_NEO4J_CONTAINER:-WeKnora-neo4j-dev}"
postgres_user="${WEKNORA_POSTGRES_USER:-postgres}"
postgres_database="${WEKNORA_POSTGRES_DATABASE:-WeKnora}"
neo4j_address="${WEKNORA_NEO4J_ADDRESS:-neo4j://localhost:7687}"
neo4j_database="${WEKNORA_NEO4J_DATABASE:-neo4j}"

run_psql() {
  docker exec "${postgres_container}" psql \
    -U "${postgres_user}" \
    -d "${postgres_database}" \
    -v ON_ERROR_STOP=1 \
    -At \
    -P pager=off \
    -c "$1"
}

run_cypher() {
  docker exec \
    -e NEO4J_USERNAME \
    -e NEO4J_PASSWORD \
    "${neo4j_container}" \
    cypher-shell \
    --address "${neo4j_address}" \
    --database "${neo4j_database}" \
    --non-interactive \
    --history disable \
    --format plain \
    "$1"
}

cleanup() {
  set +e
  run_psql "
DELETE FROM math_curriculum_edges WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}';
DELETE FROM math_curriculum_nodes WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}';" >/dev/null 2>&1
  run_cypher "
MATCH (node:MathCurriculum {tenant_id: ${tenant_id}})
WHERE node.knowledge_base_id IN ['${kb_id}', '${other_kb_id}']
DETACH DELETE node;" >/dev/null 2>&1
}
trap cleanup EXIT
cleanup

run_psql "
INSERT INTO math_curriculum_nodes (
  id, tenant_id, knowledge_base_id, parent_id, node_type, grade, term,
  domain, title, summary, sort_order, expected_question_count, metadata
) VALUES
  ('node-a', ${tenant_id}, '${kb_id}', '', 'concept', 1, 1, 'number', 'A', '', 1, 3, '{\"source\":\"integration\"}'),
  ('node-b', ${tenant_id}, '${kb_id}', '', 'concept', 1, 1, 'number', 'B', '', 2, 3, '{}'),
  ('node-c', ${tenant_id}, '${kb_id}', '', 'concept', 1, 1, 'number', 'C', '', 3, 3, '{}');
INSERT INTO math_curriculum_edges (
  id, tenant_id, knowledge_base_id, from_node_id, to_node_id,
  relation_type, strength, rationale
) VALUES (
  'edge-a', ${tenant_id}, '${kb_id}', 'node-a', 'node-b',
  'prerequisite', 1, 'integration test'
);" >/dev/null

sync_graph() {
  TMPDIR="${TMPDIR:-/tmp}" \
  WEKNORA_ENV_FILE="${env_file}" \
  WEKNORA_MATH_TENANT_ID="${tenant_id}" \
  WEKNORA_MATH_KB_ID="${kb_id}" \
  WEKNORA_POSTGRES_CONTAINER="${postgres_container}" \
  WEKNORA_NEO4J_CONTAINER="${neo4j_container}" \
  WEKNORA_POSTGRES_USER="${postgres_user}" \
  WEKNORA_POSTGRES_DATABASE="${postgres_database}" \
  WEKNORA_NEO4J_ADDRESS="${neo4j_address}" \
  WEKNORA_NEO4J_DATABASE="${neo4j_database}" \
  "${repo_root}/scripts/sync_math_mastery_graph.sh"
}

sync_graph >/dev/null
sync_graph >/dev/null

initial_state="$(run_cypher "
CALL {
  MATCH (node:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}'})
  RETURN count(node) AS nodes
}
CALL {
  MATCH ()-[relationship:CURRICULUM_EDGE {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}', edge_id: 'edge-a'}]->(target)
  RETURN count(relationship) AS relationships, collect(target.node_id) AS targets
}
RETURN nodes, relationships, targets;")"
[[ "${initial_state}" == *'3, 1, ["node-b"]'* ]]

run_cypher "
MERGE (other:MathCurriculum {
  tenant_id: ${tenant_id},
  knowledge_base_id: '${other_kb_id}',
  node_id: 'other-node'
});" >/dev/null

run_psql "
UPDATE math_curriculum_edges
SET to_node_id='node-c', updated_at=now()
WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}' AND id='edge-a';" >/dev/null
sync_graph >/dev/null

changed_endpoint_state="$(run_cypher "
CALL {
  MATCH ()-[relationship:CURRICULUM_EDGE {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}', edge_id: 'edge-a'}]->(target)
  RETURN count(relationship) AS relationships, collect(target.node_id) AS targets
}
CALL {
  MATCH (other:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${other_kb_id}', node_id: 'other-node'})
  RETURN count(other) AS other_scope_nodes
}
RETURN relationships, targets, other_scope_nodes;")"
[[ "${changed_endpoint_state}" == *'1, ["node-c"], 1'* ]]

run_psql "
UPDATE math_curriculum_edges
SET to_node_id='missing-node', updated_at=now()
WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}' AND id='edge-a';" >/dev/null
if sync_graph >/dev/null 2>&1; then
  printf 'missing edge endpoint unexpectedly produced a successful projection\n' >&2
  exit 1
fi

rolled_back_target="$(run_cypher "
MATCH ()-[relationship:CURRICULUM_EDGE {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}', edge_id: 'edge-a'}]->(target)
RETURN count(relationship) AS relationships, collect(target.node_id) AS targets;")"
[[ "${rolled_back_target}" == *'1, ["node-c"]'* ]]

run_psql "
UPDATE math_curriculum_edges
SET to_node_id='node-c', updated_at=now()
WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}' AND id='edge-a';
DELETE FROM math_curriculum_nodes
WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}' AND id='node-b';" >/dev/null
run_cypher "
MATCH (stale:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}', node_id: 'node-b'})
MATCH (other:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${other_kb_id}', node_id: 'other-node'})
MERGE (stale)-[:FOREIGN_TEST]->(other);" >/dev/null

if sync_graph >/dev/null 2>&1; then
  printf 'stale node with an out-of-scope relationship was detached unexpectedly\n' >&2
  exit 1
fi

boundary_state="$(run_cypher "
MATCH (stale:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}', node_id: 'node-b'})-[foreign:FOREIGN_TEST]->(other:MathCurriculum {knowledge_base_id: '${other_kb_id}'})
RETURN count(stale) AS stale_nodes, count(foreign) AS foreign_relationships, count(other) AS other_nodes;")"
[[ "${boundary_state}" == *'1, 1, 1'* ]]

run_cypher "
MATCH (:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}'})-[foreign:FOREIGN_TEST]->(:MathCurriculum {knowledge_base_id: '${other_kb_id}'})
DELETE foreign;" >/dev/null
sync_graph >/dev/null

final_state="$(run_cypher "
CALL {
  MATCH (node:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}'})
  RETURN count(node) AS nodes
}
CALL {
  MATCH ()-[relationship:CURRICULUM_EDGE {tenant_id: ${tenant_id}, knowledge_base_id: '${kb_id}'}]->()
  RETURN count(relationship) AS relationships
}
CALL {
  MATCH (other:MathCurriculum {tenant_id: ${tenant_id}, knowledge_base_id: '${other_kb_id}', node_id: 'other-node'})
  RETURN count(other) AS other_scope_nodes
}
RETURN nodes, relationships, other_scope_nodes;")"
[[ "${final_state}" == *'2, 1, 1'* ]]

printf 'sync_math_mastery_graph_integration_test: PASS\n'
