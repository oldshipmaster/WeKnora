CREATE TABLE IF NOT EXISTS math_curriculum_nodes (
    id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    parent_id VARCHAR(128) NOT NULL DEFAULT '',
    node_type VARCHAR(24) NOT NULL,
    grade INT NOT NULL,
    term INT NOT NULL,
    domain VARCHAR(32) NOT NULL,
    title VARCHAR(255) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    expected_question_count INT NOT NULL DEFAULT 3,
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, id)
);
CREATE INDEX IF NOT EXISTS idx_math_nodes_kb_order ON math_curriculum_nodes (tenant_id, knowledge_base_id, grade, term, sort_order);

CREATE TABLE IF NOT EXISTS math_curriculum_edges (
    id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    from_node_id VARCHAR(128) NOT NULL,
    to_node_id VARCHAR(128) NOT NULL,
    relation_type VARCHAR(32) NOT NULL,
    strength DOUBLE PRECISION NOT NULL DEFAULT 1,
    rationale TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, id)
);
CREATE INDEX IF NOT EXISTS idx_math_edges_kb_from ON math_curriculum_edges (tenant_id, knowledge_base_id, from_node_id);
CREATE INDEX IF NOT EXISTS idx_math_edges_kb_to ON math_curriculum_edges (tenant_id, knowledge_base_id, to_node_id);

CREATE TABLE IF NOT EXISTS math_source_bindings (
    id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    target_id VARCHAR(128) NOT NULL,
    node_id VARCHAR(128) NOT NULL DEFAULT '',
    knowledge_id VARCHAR(36) NOT NULL DEFAULT '',
    chunk_id VARCHAR(36) NOT NULL DEFAULT '',
    source_type VARCHAR(24) NOT NULL,
    status VARCHAR(24) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    edition VARCHAR(64) NOT NULL DEFAULT '',
    school_year INT NOT NULL DEFAULT 0,
    season VARCHAR(16) NOT NULL DEFAULT '',
    grade INT NOT NULL DEFAULT 0,
    term INT NOT NULL DEFAULT 0,
    page_from INT NOT NULL DEFAULT 0,
    page_to INT NOT NULL DEFAULT 0,
    content_hash VARCHAR(64) NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_math_sources_target ON math_source_bindings (tenant_id, knowledge_base_id, target_id);
CREATE INDEX IF NOT EXISTS idx_math_sources_status ON math_source_bindings (tenant_id, knowledge_base_id, status);

CREATE TABLE IF NOT EXISTS math_questions (
    id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    source_binding_id VARCHAR(128) NOT NULL DEFAULT '',
    question_locator VARCHAR(255) NOT NULL,
    answer_locator VARCHAR(255) NOT NULL DEFAULT '',
    question_type VARCHAR(32) NOT NULL,
    difficulty DOUBLE PRECISION NOT NULL DEFAULT 0,
    scoring_rule JSONB NOT NULL DEFAULT '{}'::JSONB,
    extraction_confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, id)
);

CREATE TABLE IF NOT EXISTS math_question_nodes (
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    question_id VARCHAR(128) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, question_id, node_id)
);

CREATE TABLE IF NOT EXISTS math_diagnostic_attempts (
    id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    profile_id VARCHAR(128) NOT NULL,
    status VARCHAR(24) NOT NULL,
    scope JSONB NOT NULL DEFAULT '{}'::JSONB,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, id)
);
CREATE INDEX IF NOT EXISTS idx_math_attempts_profile ON math_diagnostic_attempts (tenant_id, knowledge_base_id, profile_id, created_at DESC);

CREATE TABLE IF NOT EXISTS math_diagnostic_responses (
    id VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    attempt_id VARCHAR(128) NOT NULL,
    question_id VARCHAR(128) NOT NULL,
    node_id VARCHAR(128) NOT NULL,
    question_type VARCHAR(32) NOT NULL,
    correct BOOLEAN NOT NULL,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    self_confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    error_type VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, knowledge_base_id, id)
);
CREATE INDEX IF NOT EXISTS idx_math_responses_node ON math_diagnostic_responses (tenant_id, knowledge_base_id, node_id, created_at);
CREATE INDEX IF NOT EXISTS idx_math_responses_attempt ON math_diagnostic_responses (tenant_id, knowledge_base_id, attempt_id);
