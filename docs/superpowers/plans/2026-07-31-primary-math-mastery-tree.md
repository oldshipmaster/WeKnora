# Primary Math Mastery Tree Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend WeKnora with a locally hosted, evidence-backed PEP primary-school mathematics mastery tree that imports the 12 textbooks, exposes explicit 2026 exam-material gaps, and helps a parent judge whether knowledge is systematic.

**Architecture:** Keep WeKnora's existing knowledge-base, vector, Wiki, Neo4j, authentication, and upload pipelines. Add a math-mastery domain layer for curriculum DAGs, source status, diagnostic evidence, and explainable mastery summaries; render it as a new knowledge-base tab. Keep copyrighted PDFs and question text outside Git and make ingestion idempotent.

**Tech Stack:** Go 1.24, Gin, GORM, PostgreSQL/pgvector, Neo4j, Vue 3, TypeScript, TDesign, Docker Compose, DashScope OpenAI-compatible chat/embedding APIs.

## Global Constraints

- Never commit the DashScope key, textbook PDFs, exam PDFs, extracted question text, or child-identifying data.
- Use `qwen3.7-plus` for chat/extraction and `text-embedding-v4` at 1024 dimensions for embeddings.
- Never substitute 2025 exam material when the requested 2026 spring/autumn assets are missing.
- Mastery must come from evidence. A node cannot become `mastered` from a manual toggle or one answer.
- Run Go caches on `/Volumes/extfastdata01` because the macOS data volume is at 99% capacity.

---

### Task 1: Add safe runtime model and graph configuration

**Files:**
- Modify: `docker-compose.yml`
- Create: `config/builtin_models.yaml`
- Create locally only: `.env`

1. Add a failing Compose/config assertion that verifies the built-in model file is mounted and contains no literal API key.
2. Add two env-driven built-in model entries: `qwen3.7-plus` and `text-embedding-v4` with dimension 1024.
3. Mount the model file read-only into the app service and enable the existing Neo4j profile through local `.env` values.
4. Retrieve the API key from macOS Keychain into `.env`, generate a 32-byte `SYSTEM_AES_KEY`, and keep `.env` ignored.
5. Run `docker compose --profile neo4j config` and inspect that variables resolve without printing the key.
6. Commit tracked configuration only.

### Task 2: Build and test an idempotent material manifest scanner

**Files:**
- Create: `internal/mathmastery/manifest.go`
- Create: `internal/mathmastery/manifest_test.go`
- Create: `cmd/math-mastery-import/main.go`
- Create: `data/math-mastery/pep-primary-math.json`

1. Write failing tests for exact discovery of the 12 PEP math textbooks, stable SHA-256 IDs, duplicate suppression, and rejection of non-2026 exam candidates.
2. Implement typed manifest entries and target slots with statuses `found`, `missing`, `uploaded`, `processing`, `ready`, and `failed`.
3. Add a CLI `scan` command that accepts a source root and writes only metadata JSON to a caller-selected runtime path.
4. Add a repository-safe curriculum seed containing grade/term/domain/unit/concept metadata and prerequisite edges, without textbook prose or question content.
5. Run focused Go tests with external `GOMODCACHE` and `GOCACHE`.
6. Commit scanner, CLI, tests, and curriculum metadata.

### Task 3: Implement explainable mastery scoring and DAG validation

**Files:**
- Create: `internal/types/math_mastery.go`
- Create: `internal/mathmastery/graph.go`
- Create: `internal/mathmastery/graph_test.go`
- Create: `internal/mathmastery/scoring.go`
- Create: `internal/mathmastery/scoring_test.go`

1. Write failing tests for missing-node edges, graph cycles, grade ordering, and valid cross-grade prerequisites.
2. Implement deterministic DAG validation and topological ordering.
3. Write failing scoring tests proving zero evidence is `untested`, incorrect evidence is `weak`, one correct response is at most `developing`, and `mastered` requires three independent questions, two question types, and two sessions.
4. Implement score, confidence, evidence count, state, and human-readable reason calculation.
5. Run all focused domain tests.
6. Commit domain types and algorithms.

### Task 4: Persist curriculum, sources, questions, and diagnostic evidence

**Files:**
- Create: `migrations/versioned/000079_math_mastery_tree.up.sql`
- Create: `migrations/versioned/000079_math_mastery_tree.down.sql`
- Create: `internal/types/interfaces/math_mastery.go`
- Create: `internal/application/repository/math_mastery.go`
- Create: `internal/application/repository/math_mastery_test.go`

1. Write a failing repository test for upsert idempotency, ordered tree reads, source-status updates, and diagnostic response insertion.
2. Add tables and indexes for nodes, edges, source bindings, questions, question-node mappings, attempts, and responses.
3. Implement tenant- and knowledge-base-scoped repository methods using GORM transactions.
4. Verify migrations apply cleanly to the project's PostgreSQL container.
5. Run repository tests.
6. Commit migrations and persistence.

### Task 5: Expose protected math-mastery APIs

**Files:**
- Create: `internal/application/service/math_mastery.go`
- Create: `internal/application/service/math_mastery_test.go`
- Create: `internal/handler/math_mastery.go`
- Create: `internal/handler/math_mastery_test.go`
- Create: `internal/router/routes_math_mastery.go`
- Modify: `internal/container/container.go`
- Modify: `internal/router/router.go`

1. Write failing service tests for curriculum seeding, overview aggregation, blocked-node explanation, and response submission.
2. Implement service methods that enforce knowledge-base scope and call the scoring/DAG packages.
3. Write failing handler tests for unauthorized, read-only, invalid payload, and successful paths.
4. Add routes under `/api/v1/knowledge-bases/:id/math-mastery` for tree, overview, sources, seed, attempts, and responses, reusing WeKnora permissions.
5. Wire repository, service, and handler through dependency injection.
6. Run focused and full Go tests available in the environment.
7. Commit backend API work.

### Task 6: Implement the knowledge-base mastery-tree experience

**Files:**
- Create: `frontend/src/api/math-mastery/index.ts`
- Create: `frontend/src/views/knowledge/mastery/mathMasteryGraph.ts`
- Create: `frontend/src/views/knowledge/mastery/mathMasteryGraph.test.ts`
- Create: `frontend/src/views/knowledge/mastery/MathMasteryTree.vue`
- Modify: `frontend/src/views/knowledge/KnowledgeBase.vue`
- Modify: `frontend/src/i18n/locales/*.ts`

1. Write failing TypeScript tests for overview counts, filters, blocked-chain selection, node positions, and SVG edge paths.
2. Implement typed API calls and pure graph-layout/view-model helpers.
3. Build the dark layered tree with domain colors, clear focus states, reduced-motion support, responsive horizontal scrolling, prerequisite lines, filters, overview cards, and a node detail drawer.
4. Show 12 textbook slots separately from 2026 spring/autumn exam slots; render missing assets as explicit gaps.
5. Add the `mastery` tab to the knowledge-base detail view and keep all locale key sets aligned.
6. Run frontend tests, i18n audit, type-check, and a production build with a larger Node heap.
7. Commit the frontend feature.

### Task 7: Start the stack and run the real textbook ingestion pipeline

**Files:**
- Runtime only: `.env`
- Runtime only: `/Volumes/extfastdata01/WeKnora-runtime/math-mastery/manifest.json`
- Modify if needed: `cmd/math-mastery-import/main.go`

1. Start PostgreSQL, Redis, MinIO, DocReader, Neo4j, and WeKnora with `docker compose --profile neo4j up -d`.
2. Verify health endpoints, model registration, and 1024-dimensional embedding preflight.
3. Create or reuse the “人教版小学数学” knowledge base with graph and Wiki extraction enabled.
4. Scan the source directory and assert textbook coverage is exactly 12/12 while both requested 2026 exam collections remain explicit missing slots if still absent.
5. Upload the 12 textbooks idempotently, persist upload identifiers/status, and wait for parsing/vectorization completion.
6. Seed the curriculum DAG and bind detected textbook sources; start Wiki/GraphRAG generation through existing WeKnora capabilities.
7. Record failures per file and retry only failed stages.
8. Commit any ingestion code corrections, never runtime data.

### Task 8: Browser QA, diagnostic smoke test, and iterative hardening

**Files:**
- Modify only files implicated by observed failures
- Create: `docs/verification/2026-07-31-primary-math-mastery-tree.md`

1. Open the local product and verify login, knowledge-base navigation, mastery-tree rendering, filters, drawer, missing-material states, and mobile-width behavior.
2. Submit synthetic diagnostic evidence across two sessions and verify the state transitions match the scoring contract.
3. Inspect logs for secret leakage, upload failures, graph failures, and database errors.
4. Iterate on the highest-risk failures while preserving passing tests after each change.
5. Document exact commands, counts, unresolved material gaps, and screenshots/HTTP evidence.
6. Run final Go tests, frontend tests, type-check, i18n audit, production build, Compose status, API smoke checks, and `git diff --check`.
7. Commit the verification record and final fixes.

### Task 9: Publish the iteration to the user's fork

**Files:**
- No new product files required

1. Review `git status`, diff stat, and commit history for secrets or local artifacts.
2. Run a final secret scan for the DashScope key prefix and runtime paths.
3. Push `codex/math-knowledge-mastery-tree` to `oldshipmaster/WeKnora`.
4. Report the branch, commits, live local URL, ingestion counts, verification evidence, and known 2026 exam-material blocker.
