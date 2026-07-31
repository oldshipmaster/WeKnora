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

kb_id="${1:-${WEKNORA_MATH_KB_ID:-}}"
tenant_id="${WEKNORA_MATH_TENANT_ID:-10000}"
postgres_container="${WEKNORA_POSTGRES_CONTAINER:-WeKnora-postgres-dev}"
neo4j_container="${WEKNORA_NEO4J_CONTAINER:-WeKnora-neo4j-dev}"
postgres_user="${WEKNORA_POSTGRES_USER:-postgres}"
postgres_database="${WEKNORA_POSTGRES_DATABASE:-WeKnora}"
neo4j_address="${WEKNORA_NEO4J_ADDRESS:-neo4j://localhost:7687}"
neo4j_database="${WEKNORA_NEO4J_DATABASE:-neo4j}"

if [[ ! "${kb_id}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]]; then
  printf 'WEKNORA_MATH_KB_ID or the first argument must be a safe knowledge-base id\n' >&2
  exit 2
fi
if [[ ! "${tenant_id}" =~ ^[0-9]+$ ]]; then
  printf 'WEKNORA_MATH_TENANT_ID must be numeric\n' >&2
  exit 2
fi
: "${NEO4J_USERNAME:?NEO4J_USERNAME is required}"
: "${NEO4J_PASSWORD:?NEO4J_PASSWORD is required}"
export NEO4J_USERNAME NEO4J_PASSWORD

projection_params="$(docker exec "${postgres_container}" psql -U "${postgres_user}" -d "${postgres_database}" \
  -v ON_ERROR_STOP=1 -At -P pager=off -c "
SELECT json_build_object(
  'tenant_id', ${tenant_id},
  'knowledge_base_id', '${kb_id}',
  'node_ids', COALESCE((
    SELECT json_agg(id ORDER BY id)
    FROM math_curriculum_nodes
    WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}'
  ), '[]'::json),
  'edge_ids', COALESCE((
    SELECT json_agg(id ORDER BY id)
    FROM math_curriculum_edges
    WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}'
  ), '[]'::json),
  'nodes', COALESCE((
    SELECT json_agg(json_build_object(
      'id', id,
      'parent_id', parent_id,
      'node_type', node_type,
      'grade', grade,
      'term', term,
      'domain', domain,
      'title', title,
      'summary', summary,
      'sort_order', sort_order,
      'expected_question_count', expected_question_count,
      'metadata', metadata
    ) ORDER BY grade, term, sort_order, id)
    FROM math_curriculum_nodes
    WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}'
  ), '[]'::json),
  'edges', COALESCE((
    SELECT json_agg(json_build_object(
      'id', id,
      'from_node_id', from_node_id,
      'to_node_id', to_node_id,
      'relation_type', relation_type,
      'strength', strength,
      'rationale', rationale
    ) ORDER BY id)
    FROM math_curriculum_edges
    WHERE tenant_id=${tenant_id} AND knowledge_base_id='${kb_id}'
  ), '[]'::json)
);")"

if [[ -z "${projection_params}" ]]; then
  printf 'Postgres returned no math curriculum projection data\n' >&2
  exit 3
fi
if ! printf '%s' "${projection_params}" | jq -e \
  --arg kb_id "${kb_id}" \
  --argjson tenant_id "${tenant_id}" \
  '.knowledge_base_id == $kb_id and .tenant_id == $tenant_id and (.nodes | type == "array" and length > 0) and (.edges | type == "array")' \
  >/dev/null; then
  printf 'Postgres returned an invalid or empty math curriculum projection\n' >&2
  exit 3
fi
projection_json_string="$(printf '%s' "${projection_params}" | jq -Rs .)"

read -r -d '' cypher <<'CYPHER' || true
WITH apoc.convert.fromJsonMap($payload) AS projection
CALL {
  WITH projection
  WITH projection, projection.nodes AS rows
  UNWIND rows AS row
  MERGE (node:MathCurriculum {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id,
    node_id: row.id
  })
  SET node.parent_id = row.parent_id,
      node.node_type = row.node_type,
      node.grade = row.grade,
      node.term = row.term,
      node.domain = row.domain,
      node.title = row.title,
      node.summary = row.summary,
      node.sort_order = row.sort_order,
      node.expected_question_count = row.expected_question_count,
      node.metadata = apoc.convert.toJson(row.metadata)
  RETURN count(*) AS nodes_upserted
}
CALL {
  WITH projection
  MATCH ()-[relationship:CURRICULUM_EDGE {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id
  }]->()
  DELETE relationship
  RETURN count(*) AS relationships_replaced
}
CALL {
  WITH projection
  WITH projection, projection.edges AS rows
  UNWIND rows AS row
  MATCH (source:MathCurriculum {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id,
    node_id: row.from_node_id
  })
  MATCH (target:MathCurriculum {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id,
    node_id: row.to_node_id
  })
  MERGE (source)-[relationship:CURRICULUM_EDGE {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id,
    edge_id: row.id
  }]->(target)
  SET relationship.relation_type = row.relation_type,
      relationship.strength = row.strength,
      relationship.rationale = row.rationale
  RETURN count(*) AS edges_upserted
}
CALL {
  WITH projection
  WITH projection, projection.node_ids AS valid_node_ids
  MATCH (node:MathCurriculum {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id
  })
  WHERE NOT node.node_id IN valid_node_ids
  DELETE node
  RETURN count(*) AS stale_nodes_deleted
}
CALL {
  WITH projection
  MATCH (node:MathCurriculum {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id
  })
  RETURN count(node) AS projected_nodes
}
CALL {
  WITH projection
  MATCH ()-[relationship:CURRICULUM_EDGE {
    tenant_id: projection.tenant_id,
    knowledge_base_id: projection.knowledge_base_id
  }]->()
  RETURN count(relationship) AS projected_relationships
}
WITH projection,
     nodes_upserted,
     relationships_replaced,
     edges_upserted,
     stale_nodes_deleted,
     projected_nodes,
     projected_relationships
CALL apoc.util.validate(
  projected_nodes <> size(projection.nodes) OR projected_relationships <> size(projection.edges),
  'math mastery graph projection count mismatch',
  []
)
RETURN nodes_upserted,
       relationships_replaced,
       edges_upserted,
       stale_nodes_deleted,
       projected_nodes,
       projected_relationships;
CYPHER

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
  -P "{payload: ${projection_json_string}}" \
  "${cypher}"
