package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// knowledgeTestDDL is the minimal subset of the knowledge schema this
// suite needs. We avoid AutoMigrate because Knowledge carries multiple
// JSONB-tagged fields whose SQLite mapping is fragile.
//
// Table name is `knowledges` (plural) — that's what migration 000000
// creates and what GORM's default pluralization expects when the
// service code uses Model(&types.Knowledge{}).
const knowledgeTestDDL = `
CREATE TABLE IF NOT EXISTS knowledges (
    id              VARCHAR(64) PRIMARY KEY,
    tenant_id       INTEGER NOT NULL DEFAULT 0,
    knowledge_base_id VARCHAR(64),
    parse_status    VARCHAR(32) NOT NULL DEFAULT 'pending',
    summary_status  VARCHAR(32) NOT NULL DEFAULT 'none',
    pending_subtasks_count INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,
    title           TEXT,
    file_type       TEXT,
    enable_status   TEXT NOT NULL DEFAULT 'enabled',
    type            TEXT NOT NULL DEFAULT 'document',
    embedding_model_id TEXT NOT NULL DEFAULT '',
    storage_size    BIGINT NOT NULL DEFAULT 0,
    processed_at    DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME
);
`

const housekeepingSpansDDL = `
CREATE TABLE IF NOT EXISTS knowledge_processing_spans (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_id    VARCHAR(64) NOT NULL,
    attempt         INTEGER     NOT NULL DEFAULT 1,
    span_id         VARCHAR(64) NOT NULL,
    parent_span_id  VARCHAR(64),
    name            VARCHAR(255) NOT NULL,
    kind            VARCHAR(16) NOT NULL,
    status          VARCHAR(16) NOT NULL,
    input           TEXT,
    output          TEXT,
    metadata        TEXT,
    error_code      VARCHAR(64),
    error_message   TEXT,
    error_detail    TEXT,
    started_at      DATETIME,
    finished_at     DATETIME,
    duration_ms     BIGINT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (knowledge_id, attempt, span_id)
);
`

const housekeepingPendingOpsDDL = `
CREATE TABLE IF NOT EXISTS task_pending_ops (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id   INTEGER NOT NULL,
    task_type   VARCHAR(64) NOT NULL,
    scope       VARCHAR(32) NOT NULL,
    scope_id    VARCHAR(64) NOT NULL,
    op          VARCHAR(32) NOT NULL,
    dedup_key   VARCHAR(128) NOT NULL DEFAULT '',
    payload     TEXT NOT NULL DEFAULT '{}',
    fail_count  INTEGER NOT NULL DEFAULT 0,
    enqueued_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    claimed_at  DATETIME
);
`

func setupHousekeepingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(knowledgeTestDDL).Error)
	require.NoError(t, db.Exec(housekeepingSpansDDL).Error)
	require.NoError(t, db.Exec(housekeepingPendingOpsDDL).Error)
	return db
}

// insertKnowledge writes a knowledge row at the given updated_at. We
// can't pass updated_at through GORM defaults since CURRENT_TIMESTAMP
// would override our test fixture; raw SQL keeps the timestamp.
func insertKnowledge(t *testing.T, db *gorm.DB, id, status string, updatedAt time.Time) {
	t.Helper()
	pendingSubtasks := 0
	if status == types.ParseStatusFinalizing {
		pendingSubtasks = 1
	}
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, parse_status, pending_subtasks_count, updated_at) VALUES (?, ?, ?, ?)`,
		id, status, pendingSubtasks, updatedAt,
	).Error)
}

func insertSpan(t *testing.T, db *gorm.DB, kid string, attempt int, spanID, status string, updatedAt time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO knowledge_processing_spans (knowledge_id, attempt, span_id, name, kind, status, updated_at)
		 VALUES (?, ?, ?, 'docreader', 'stage', ?, ?)`,
		kid, attempt, spanID, status, updatedAt,
	).Error)
}

// fakeTaskInspector is a controllable TaskInspector for the housekeeping
// suite. queued maps knowledge_id → "still has a queued task"; err forces
// the probe to fail so the fail-safe branch can be exercised.
type fakeTaskInspector struct {
	queued map[string]bool
	err    error
}

type fakeKnowledgeBaseTaskInspector struct {
	fakeTaskInspector
	queuedKB map[string]bool
	errKB    error
}

func (f fakeKnowledgeBaseTaskInspector) HasQueuedTasksForKnowledgeBase(
	_ context.Context, knowledgeBaseID string,
) (bool, error) {
	if f.errKB != nil {
		return false, f.errKB
	}
	return f.queuedKB[knowledgeBaseID], nil
}

func (f fakeTaskInspector) CancelTasksForKnowledge(
	_ context.Context, _ string,
) (int, int, error) {
	return 0, 0, nil
}

func (f fakeTaskInspector) HasQueuedTasksForKnowledge(
	_ context.Context, knowledgeID string,
) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.queued[knowledgeID], nil
}

func (f fakeTaskInspector) QueueStats(
	_ context.Context,
) ([]types.QueueStat, bool, error) {
	return nil, false, nil
}

func (f fakeTaskInspector) WorkerServerStats(
	_ context.Context,
) ([]types.WorkerServerStat, bool, error) {
	return nil, false, nil
}

func newHousekeepingSvcForTest(db *gorm.DB) *HousekeepingService {
	return newHousekeepingSvcWithInspector(db, fakeTaskInspector{})
}

func newHousekeepingSvcWithInspector(db *gorm.DB, inspector interfaces.TaskInspector) *HousekeepingService {
	cfg := &config.Config{KnowledgeBase: &config.KnowledgeBaseConfig{
		// 1h floor + 10min buffer = 70min cutoff. Tight enough to keep
		// the test's relative timestamps in seconds; the production
		// default of 2h+10min is just a constant scale factor.
		DocumentProcessTimeout: 1 * time.Hour,
	}}
	return NewHousekeepingService(db, cfg, inspector)
}

// TestHousekeeping_RecoversAbandoned exercises the happy path: a
// knowledge stuck at "processing" with no recent heartbeat (no spans,
// stale knowledge.updated_at) MUST be flipped to failed.
func TestHousekeeping_RecoversAbandoned(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour) // well past 70min cutoff
	insertKnowledge(t, db, "kid-abandoned", types.ParseStatusProcessing, stale)

	svc.runSweep(context.Background())

	var status, errMsg string
	require.NoError(t, db.Raw(
		`SELECT parse_status, error_message FROM knowledges WHERE id = ?`, "kid-abandoned",
	).Row().Scan(&status, &errMsg))
	assert.Equal(t, types.ParseStatusFailed, status)
	assert.Contains(t, errMsg, "stuck in processing")
}

func TestHousekeeping_RecoversPendingTaskMissingFromQueue(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	insertKnowledge(t, db, "kid-pending-orphan", types.ParseStatusPending, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-pending-orphan",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"a stale pending row with no queue task must not remain pending forever")
}

func TestHousekeeping_PromotesFinalizingWithZeroOutstandingSubtasks(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges
		 (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		"kid-zero-finalizing", 17, "kb-owner", types.ParseStatusFinalizing, stale,
	).Error)

	svc.runSweep(context.Background())

	var status string
	var processedAt *time.Time
	require.NoError(t, db.Raw(
		`SELECT parse_status, processed_at FROM knowledges WHERE id = ?`, "kid-zero-finalizing",
	).Row().Scan(&status, &processedAt))
	assert.Equal(t, types.ParseStatusCompleted, status,
		"a successful counter decrement must not be turned into a housekeeping failure when promotion was interrupted")
	assert.NotNil(t, processedAt)
}

func TestHousekeeping_FinalFailureGuardDoesNotClobberConcurrentCompletion(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	cutoff := time.Now().Add(-svc.staleThreshold())
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges
		 (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-finalize-race", 17, "kb-owner", types.ParseStatusFinalizing, stale,
	).Error)
	candidate := types.Knowledge{
		ID: "kid-finalize-race", TenantID: 17, KnowledgeBaseID: "kb-owner",
		ParseStatus: types.ParseStatusFinalizing,
	}

	// Simulate the last subtask decrementing after the stale scan formed its
	// candidate list but before Housekeeping performs the guarded failure write.
	require.NoError(t, db.Model(&types.Knowledge{}).
		Where("id = ?", candidate.ID).
		Update("pending_subtasks_count", 0).Error)
	recovered, err := svc.markStuckKnowledgeFailed(
		context.Background(), []types.Knowledge{candidate}, svc.staleThreshold(), cutoff,
	)
	require.NoError(t, err)
	assert.Zero(t, recovered)

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, candidate.ID,
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"the stale failure write must not win after the authoritative counter reaches zero")

	svc.runSweep(context.Background())
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, candidate.ID,
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusCompleted, status,
		"the next invariant repair promotes the completed counter state")
}

func TestHousekeeping_FinalFailureGuardDoesNotClobberConcurrentWikiClaim(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	cutoff := time.Now().Add(-svc.staleThreshold())
	orphaned := time.Now().Add(-25 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges
		 (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-wiki-claim-race", 17, "kb-owner", types.ParseStatusFinalizing, stale,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload, enqueued_at, claimed_at)
		 VALUES (?, ?, ?, ?, 'ingest', ?, '{}', ?, ?)`,
		17, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, "kb-owner", "kid-wiki-claim-race",
		orphaned, orphaned,
	).Error)
	candidate := types.Knowledge{
		ID: "kid-wiki-claim-race", TenantID: 17, KnowledgeBaseID: "kb-owner",
		ParseStatus: types.ParseStatusFinalizing,
	}

	// Simulate the durable-op filter accepting an expired owner, followed by
	// the Wiki worker claiming it before Housekeeping performs the failure
	// write. The database-side ownership guard must observe the fresh claim.
	require.NoError(t, db.Model(&types.TaskPendingOp{}).
		Where("tenant_id = ? AND dedup_key = ?", 17, candidate.ID).
		Update("claimed_at", time.Now()).Error)
	recovered, err := svc.markStuckKnowledgeFailed(
		context.Background(), []types.Knowledge{candidate}, svc.staleThreshold(), cutoff,
	)
	require.NoError(t, err)
	assert.Zero(t, recovered)

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, candidate.ID,
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"the final stale write must not overwrite a newly claimed durable Wiki operation")

	svc.runSweep(context.Background())
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, candidate.ID,
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"subsequent sweeps must preserve the fresh durable Wiki owner")
}

func TestHousekeeping_FinalFailureGuardDoesNotClobberConcurrentProgress(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	cutoff := time.Now().Add(-svc.staleThreshold())
	orphaned := time.Now().Add(-25 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges
		 (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 2, ?)`,
		"kid-wiki-progress-race", 17, "kb-owner", types.ParseStatusFinalizing, stale,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload, enqueued_at, claimed_at)
		 VALUES (?, ?, ?, ?, 'ingest', ?, '{}', ?, ?)`,
		17, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, "kb-owner", "kid-wiki-progress-race",
		orphaned, orphaned,
	).Error)
	candidate := types.Knowledge{
		ID: "kid-wiki-progress-race", TenantID: 17, KnowledgeBaseID: "kb-owner",
		ParseStatus: types.ParseStatusFinalizing,
	}

	// Simulate one of two enrichment subtasks completing after the stale
	// candidate list formed. It refreshes the knowledge heartbeat and settles
	// its durable operation while another subtask remains outstanding.
	require.NoError(t, db.Model(&types.Knowledge{}).
		Where("id = ?", candidate.ID).
		Updates(map[string]interface{}{
			"pending_subtasks_count": 1,
			"updated_at":             time.Now(),
		}).Error)
	require.NoError(t, db.Where(
		"tenant_id = ? AND dedup_key = ?", 17, candidate.ID,
	).Delete(&types.TaskPendingOp{}).Error)

	recovered, err := svc.markStuckKnowledgeFailed(
		context.Background(), []types.Knowledge{candidate}, svc.staleThreshold(), cutoff,
	)
	require.NoError(t, err)
	assert.Zero(t, recovered)

	var status string
	var remaining int
	require.NoError(t, db.Raw(
		`SELECT parse_status, pending_subtasks_count FROM knowledges WHERE id = ?`, candidate.ID,
	).Row().Scan(&status, &remaining))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"a fresh knowledge heartbeat must invalidate an earlier stale candidate")
	assert.Equal(t, 1, remaining)
}

func TestHousekeeping_FinalFailureGuardDoesNotClobberConcurrentSpanHeartbeat(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	cutoff := time.Now().Add(-svc.staleThreshold())
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges
		 (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-span-race", 17, "kb-owner", types.ParseStatusProcessing, stale,
	).Error)
	insertSpan(t, db, "kid-span-race", 1, "docreader-stale", types.SpanStatusRunning, stale)
	candidate := types.Knowledge{
		ID: "kid-span-race", TenantID: 17, KnowledgeBaseID: "kb-owner",
		ParseStatus: types.ParseStatusProcessing,
	}

	// Simulate a worker resuming after the stale candidate list and queue
	// probes formed. SpanTracker refreshes only the span heartbeat, so the
	// final database write must re-evaluate it independently of knowledge.updated_at.
	insertSpan(t, db, "kid-span-race", 2, "docreader-resumed", types.SpanStatusRunning, time.Now())
	recovered, err := svc.markStuckKnowledgeFailed(
		context.Background(), []types.Knowledge{candidate}, svc.staleThreshold(), cutoff,
	)
	require.NoError(t, err)
	assert.Zero(t, recovered)

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, candidate.ID,
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusProcessing, status,
		"a fresh span heartbeat must invalidate an earlier stale candidate")
}

func TestHousekeeping_PreservesPendingTaskStillQueued(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcWithInspector(db, fakeTaskInspector{
		queued: map[string]bool{"kid-pending-queued": true},
	})
	stale := time.Now().Add(-3 * time.Hour)
	insertKnowledge(t, db, "kid-pending-queued", types.ParseStatusPending, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-pending-queued",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusPending, status,
		"backlogged pending work remains owned by the durable queue")
}

// TestHousekeeping_NoFalseKill_ActiveSpan is the regression test for
// the "long DocReader silently runs longer than DocumentProcessTimeout"
// scenario the user flagged. A knowledge whose knowledge.updated_at
// looks stale BUT whose span tree shows recent activity must NOT be
// killed.
func TestHousekeeping_NoFalseKill_ActiveSpan(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	insertKnowledge(t, db, "kid-active", types.ParseStatusProcessing, stale)
	// Span heartbeat well within the 70min cutoff — it represents
	// "we're STILL working, the worker just hasn't transitioned the
	// parse_status column yet".
	insertSpan(t, db, "kid-active", 1, "docreader-1", types.SpanStatusRunning, time.Now().Add(-2*time.Minute))

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-active",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusProcessing, status,
		"knowledge with recent span heartbeat must NOT be flipped to failed")
}

// TestHousekeeping_NoFalseKill_StaleSpanRecovers confirms the inverse:
// a knowledge whose span tree has ALSO gone silent past the threshold
// is genuinely stuck and must be recovered.
func TestHousekeeping_NoFalseKill_StaleSpanRecovers(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	insertKnowledge(t, db, "kid-stuck", types.ParseStatusProcessing, stale)
	// Span row stale by the same amount — no recent activity anywhere.
	insertSpan(t, db, "kid-stuck", 1, "docreader-1", types.SpanStatusRunning, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-stuck",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"genuinely stuck knowledge (knowledge AND spans both stale) must still be recovered")
}

// TestHousekeeping_NoFalseKill_TasksStillQueued is the regression test
// for the backpressure case: a finalizing row whose span heartbeat has
// gone stale (enrichment subtasks fanned out but no worker has picked
// them up yet) must NOT be killed while its tasks are still queued.
func TestHousekeeping_NoFalseKill_TasksStillQueued(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcWithInspector(db, fakeTaskInspector{
		queued: map[string]bool{"kid-backlogged": true},
	})
	stale := time.Now().Add(-3 * time.Hour)
	// finalizing + stale knowledge + stale span: span-only heuristics
	// would flag this as stuck, but the queue still holds its subtasks.
	insertKnowledge(t, db, "kid-backlogged", types.ParseStatusFinalizing, stale)
	insertSpan(t, db, "kid-backlogged", 1, "post-1", types.SpanStatusRunning, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-backlogged",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"finalizing row with tasks still queued must NOT be flipped to failed")
}

func TestHousekeeping_NoFalseKill_FinalizingWithDurableWikiOp(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-wiki", 17, "kb-wiki", types.ParseStatusFinalizing, stale,
	).Error)
	insertSpan(t, db, "kid-wiki", 1, "post-wiki", types.SpanStatusRunning, stale)
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload, enqueued_at, claimed_at)
		 VALUES (?, ?, ?, ?, 'ingest', ?, '{}', ?, ?)`,
		17, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, "kb-wiki", "kid-wiki",
		time.Now().Add(-25*time.Hour), time.Now(),
	).Error)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-wiki",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"a durable wiki op still owns the finalizing knowledge even though its Redis trigger is KB-scoped")
}

func TestHousekeeping_PreservesOldDurableWikiBacklogWithLiveKBTrigger(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcWithInspector(db, fakeKnowledgeBaseTaskInspector{
		queuedKB: map[string]bool{"kb-wiki": true},
	})
	stale := time.Now().Add(-3 * time.Hour)
	orphaned := time.Now().Add(-25 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-wiki-backlog", 17, "kb-wiki", types.ParseStatusFinalizing, stale,
	).Error)
	insertSpan(t, db, "kid-wiki-backlog", 1, "post-wiki-backlog", types.SpanStatusRunning, stale)
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload, enqueued_at, claimed_at)
		 VALUES (?, ?, ?, ?, 'ingest', ?, '{}', ?, ?)`,
		17, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, "kb-wiki", "kid-wiki-backlog", orphaned, orphaned,
	).Error)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-wiki-backlog",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFinalizing, status,
		"a verified KB-scoped Wiki trigger keeps a legitimate large backlog alive past the age ceiling")
}

func TestHousekeeping_RecoversFinalizingWithOrphanedDurableWikiOp(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	orphaned := time.Now().Add(-25 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-wiki-orphan", 17, "kb-wiki", types.ParseStatusFinalizing, stale,
	).Error)
	insertSpan(t, db, "kid-wiki-orphan", 1, "post-wiki-orphan", types.SpanStatusRunning, stale)
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload, enqueued_at, claimed_at)
		 VALUES (?, ?, ?, ?, 'ingest', ?, '{}', ?, ?)`,
		17, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, "kb-wiki", "kid-wiki-orphan", orphaned, orphaned,
	).Error)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-wiki-orphan",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"an expired durable Wiki owner must not suppress the final stuck-task safety net forever")
}

func TestHousekeeping_DurableWikiOwnershipIsScopeIsolated(t *testing.T) {
	tests := []struct {
		name     string
		tenantID uint64
		taskType string
		scope    string
		scopeID  string
		op       string
	}{
		{name: "wrong tenant", tenantID: 18, taskType: types.TypeWikiIngest, scope: types.TaskScopeKnowledgeBase, scopeID: "kb-owner", op: WikiOpIngest},
		{name: "wrong knowledge base", tenantID: 17, taskType: types.TypeWikiIngest, scope: types.TaskScopeKnowledgeBase, scopeID: "kb-other", op: WikiOpIngest},
		{name: "wrong task type", tenantID: 17, taskType: types.TypeWikiFinalize, scope: types.TaskScopeKnowledgeBase, scopeID: "kb-owner", op: WikiOpIngest},
		{name: "wrong scope", tenantID: 17, taskType: types.TypeWikiIngest, scope: types.TaskScopeKnowledge, scopeID: "kb-owner", op: WikiOpIngest},
		{name: "wrong operation", tenantID: 17, taskType: types.TypeWikiIngest, scope: types.TaskScopeKnowledgeBase, scopeID: "kb-owner", op: WikiOpRetract},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupHousekeepingDB(t)
			svc := newHousekeepingSvcForTest(db)
			stale := time.Now().Add(-3 * time.Hour)
			require.NoError(t, db.Exec(
				`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
				 VALUES (?, ?, ?, ?, 1, ?)`,
				"kid-scope", 17, "kb-owner", types.ParseStatusFinalizing, stale,
			).Error)
			insertSpan(t, db, "kid-scope", 1, "post-scope", types.SpanStatusRunning, stale)
			require.NoError(t, db.Exec(
				`INSERT INTO task_pending_ops
				 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload)
				 VALUES (?, ?, ?, ?, ?, ?, '{}')`,
				tt.tenantID, tt.taskType, tt.scope, tt.scopeID, tt.op, "kid-scope",
			).Error)

			svc.runSweep(context.Background())

			var status string
			require.NoError(t, db.Raw(
				`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-scope",
			).Row().Scan(&status))
			assert.Equal(t, types.ParseStatusFailed, status,
				"durable Wiki ownership must match every routing dimension")
		})
	}
}

func TestHousekeeping_DurableWikiOpDoesNotOwnProcessingStage(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-processing", 17, "kb-owner", types.ParseStatusProcessing, stale,
	).Error)
	insertSpan(t, db, "kid-processing", 1, "docreader-processing", types.SpanStatusRunning, stale)
	require.NoError(t, db.Exec(
		`INSERT INTO task_pending_ops
		 (tenant_id, task_type, scope, scope_id, op, dedup_key, payload)
		 VALUES (?, ?, ?, ?, 'ingest', ?, '{}')`,
		17, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, "kb-owner", "kid-processing",
	).Error)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-processing",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"a Wiki op cannot mask a stuck primary parse stage")
}

func TestHousekeeping_DurableWikiOwnershipQueryFailureRecovers(t *testing.T) {
	db := setupHousekeepingDB(t)
	require.NoError(t, db.Migrator().DropTable(&types.TaskPendingOp{}))
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-query-failure", 17, "kb-owner", types.ParseStatusFinalizing, stale,
	).Error)
	insertSpan(t, db, "kid-query-failure", 1, "post-query-failure", types.SpanStatusRunning, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-query-failure",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"ownership lookup failure follows the existing fail-open stuck-task recovery direction")
}

func TestHousekeeping_SpanHeartbeatTableMissingRecovers(t *testing.T) {
	db := setupHousekeepingDB(t)
	require.NoError(t, db.Migrator().DropTable(&types.KnowledgeProcessingSpan{}))
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	insertKnowledge(t, db, "kid-span-table-missing", types.ParseStatusProcessing, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-span-table-missing",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"old databases without the span table retain the compatibility recovery behavior")
}

func TestHousekeeping_BothOwnershipTablesMissingRecoversFinalizing(t *testing.T) {
	db := setupHousekeepingDB(t)
	require.NoError(t, db.Migrator().DropTable(&types.TaskPendingOp{}))
	require.NoError(t, db.Migrator().DropTable(&types.KnowledgeProcessingSpan{}))
	svc := newHousekeepingSvcForTest(db)
	stale := time.Now().Add(-3 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges
		 (id, tenant_id, knowledge_base_id, parse_status, pending_subtasks_count, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?)`,
		"kid-both-tables-missing", 17, "kb-owner", types.ParseStatusFinalizing, stale,
	).Error)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-both-tables-missing",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"the compatibility fallback cumulatively disables both unavailable guards")
}

// TestHousekeeping_QueueProbeError_FailsSafe confirms the fail-safe
// direction: when the queue probe errors we still recover the row rather
// than leaving it stranded forever.
func TestHousekeeping_QueueProbeError_FailsSafe(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcWithInspector(db, fakeTaskInspector{
		err: errors.New("redis unavailable"),
	})
	stale := time.Now().Add(-3 * time.Hour)
	insertKnowledge(t, db, "kid-probeerr", types.ParseStatusProcessing, stale)

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-probeerr",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusFailed, status,
		"queue probe error must fail safe and still recover the stuck row")
}

// TestHousekeeping_PreservesRecentlyTouched: any knowledge whose
// updated_at is within the cutoff is left alone — that's the cheap
// fast path that doesn't even consult the spans table.
func TestHousekeeping_PreservesRecentlyTouched(t *testing.T) {
	db := setupHousekeepingDB(t)
	svc := newHousekeepingSvcForTest(db)
	insertKnowledge(t, db, "kid-fresh", types.ParseStatusProcessing, time.Now().Add(-30*time.Second))

	svc.runSweep(context.Background())

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "kid-fresh",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusProcessing, status,
		"knowledge updated within the cutoff must be left alone")
}
