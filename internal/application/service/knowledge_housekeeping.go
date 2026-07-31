// Package service: knowledge housekeeping.
//
// HousekeepingService periodically scans for knowledge rows that have been
// stuck in "processing" longer than any reasonable execution window and
// marks them as failed. This is the safety net that catches anything the
// other defences (asynq retry, dead-letter callback, image_multimodal
// finalize-on-last-attempt) miss — for example:
//
//   - Worker process killed mid-handler before any defer could run.
//   - DocReader call genuinely exceeding DocReaderCallTimeout AND the
//     worker subsequently being lost before retry kicks in.
//   - Multimodal Redis counter set to N but ALL N image tasks failing in
//     ways that bypass finalize (extremely rare; defence-in-depth here).
//
// Without this sweep, a single unlucky failure mode can leave a knowledge
// row in "processing" forever — invisible to users except as a permanent
// spinner. With this sweep the worst-case latency from stall to user-
// visible failure is bounded to ~1 stale-threshold + 1 sweep interval.
package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// durableWikiOwnershipMaxAge bounds how long a durable Wiki row can suppress
// the generic stuck-knowledge safety net without a fresh claim. Normal Wiki
// retry/dead-letter processing settles well inside this window; keeping a
// finite ceiling ensures a malformed or permanently orphaned row cannot mask a
// finalizing knowledge forever.
const durableWikiOwnershipMaxAge = 24 * time.Hour

// HousekeepingService runs background sweeps to recover stuck rows.
type HousekeepingService struct {
	db   *gorm.DB
	cfg  *config.Config
	cron *cron.Cron

	// inspector lets the sweep distinguish a genuinely orphaned row from
	// one whose enrichment subtasks are merely backlogged behind a busy
	// queue (no span heartbeat yet because no worker has picked them up).
	// nil-safe — a nil inspector disables the queue check and the sweep
	// falls back to the span/updated_at heuristics alone.
	inspector interfaces.TaskInspector

	mu      sync.Mutex
	started bool
}

// NewHousekeepingService constructs a HousekeepingService. It does NOT start
// the cron — call Start in the application bootstrap so a misconfigured
// cron schedule cannot prevent the rest of the service from coming up.
func NewHousekeepingService(
	db *gorm.DB, cfg *config.Config, inspector interfaces.TaskInspector,
) *HousekeepingService {
	return &HousekeepingService{
		db:        db,
		cfg:       cfg,
		inspector: inspector,
		cron: cron.New(cron.WithSeconds(), cron.WithChain(
			cron.Recover(cron.DefaultLogger),
		)),
	}
}

// Start registers the sweep schedule and begins the background runner.
// Idempotent — repeated calls are a no-op so wiring code can call Start
// without coordinating ordering.
func (h *HousekeepingService) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if !housekeepingEnabled() {
		logger.Infof(ctx, "[Housekeeping] disabled via WEKNORA_HOUSEKEEPING_ENABLED=false")
		return nil
	}
	// Every 5 minutes — frequent enough that user-visible recovery latency
	// is acceptable, infrequent enough that the SQL sweep is invisible to
	// query load even on large knowledge tables.
	if _, err := h.cron.AddFunc("0 */5 * * * *", func() {
		// Use Background so a cancelled bootstrap ctx doesn't stop sweeps.
		h.runSweep(context.Background())
	}); err != nil {
		return err
	}
	h.cron.Start()
	h.started = true
	logger.Infof(ctx, "[Housekeeping] started with 5-minute sweep")
	return nil
}

// Stop halts the cron and waits for in-flight sweeps to finish.
func (h *HousekeepingService) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.started {
		return
	}
	c := h.cron.Stop()
	<-c.Done()
	h.started = false
}

// runSweep is exported on the type for testability — tests can drive a
// single sweep without waiting for the cron tick.
func (h *HousekeepingService) runSweep(ctx context.Context) {
	threshold := h.staleThreshold()
	now := time.Now()
	cutoff := now.Add(-threshold)

	// FinalizeSubtask deliberately uses two portable guarded UPDATEs: first
	// decrement the counter, then promote finalizing -> completed. A transient
	// DB failure between those statements can leave the authoritative counter
	// at zero while the status still says finalizing. That is completed work,
	// not a stuck failure. Repair the invariant before the stale scan, and keep
	// zero-count rows out of the failure candidates if this best-effort write
	// itself hits another transient error; the next sweep will retry it.
	zeroFinalizing := h.db.WithContext(ctx).Model(&types.Knowledge{}).
		Where("parse_status = ? AND pending_subtasks_count = 0", types.ParseStatusFinalizing).
		Updates(map[string]interface{}{
			"parse_status": types.ParseStatusCompleted,
			"processed_at": now,
			"updated_at":   now,
		})
	if zeroFinalizing.Error != nil {
		logger.Warnf(ctx, "[Housekeeping] zero-subtask finalizing repair failed: %v", zeroFinalizing.Error)
	} else if zeroFinalizing.RowsAffected > 0 {
		logger.Infof(ctx, "[Housekeeping] promoted %d zero-subtask finalizing rows", zeroFinalizing.RowsAffected)
	}

	// Span rows are observability data, while knowledge.parse_status is the
	// authoritative lifecycle. A worker interruption after committing the
	// terminal knowledge state can leave an earlier span looking "running"
	// forever even though no task remains. Reconcile only running spans owned by
	// terminal knowledge: completed work closes as done; work whose parent
	// failed closes as cancelled. Pending placeholders and every non-terminal
	// knowledge remain untouched.
	h.closeTerminalKnowledgeSpans(ctx, now)

	// Sweep A: knowledge stuck in "pending", "processing", or "finalizing".
	//
	// Two-stage check is critical here: knowledge.updated_at advances
	// only at parse_status transitions, but a long stage (DocReader on
	// a 500MB PDF, embedding 5K chunks) can run for an hour with no
	// status change. Using updated_at alone would falsely kill that
	// run. So we OR-combine knowledge.updated_at with the most recent
	// span row's updated_at — every Begin/End/Fail/Skip from
	// SpanTracker bumps the span row, so an actively-progressing
	// pipeline always has a recent span heartbeat even when the
	// parent knowledge row is "frozen" mid-stage.
	//
	// Knowledge rows with no spans at all (lite mode, in-flight tasks
	// from before this code shipped) fall back to the simple
	// updated_at check — they have no heartbeat to consult.
	// Include 'pending' so a task whose enqueue was lost is eventually
	// recovered too; filterOutQueued below protects legitimately backlogged
	// tasks. Include 'finalizing' alongside 'processing': finalizing rows still
	// consume LLM compute via enrichment subtasks (summary/question/graph),
	// and the same stall modes (subtask worker dies, retry budget exhausted
	// without decrementing the counter) leave the row hanging just as
	// visibly. Housekeeping promotes both states to 'failed' once the
	// span heartbeat is older than the threshold.
	var candidates []types.Knowledge
	if err := h.db.WithContext(ctx).
		Where("parse_status IN ? AND updated_at < ?",
			[]string{types.ParseStatusPending, types.ParseStatusProcessing, types.ParseStatusFinalizing}, cutoff).
		Where("NOT (parse_status = ? AND pending_subtasks_count = 0)", types.ParseStatusFinalizing).
		Find(&candidates).Error; err != nil {
		logger.Warnf(ctx, "[Housekeeping] knowledge candidate query failed: %v", err)
		return
	}

	stuck := h.filterByLastSpanActivity(ctx, candidates, cutoff)
	spanSkipped := len(candidates) - len(stuck)

	// Wiki work is persisted in task_pending_ops by knowledge ID, while its
	// ephemeral Asynq trigger is KB-scoped and therefore carries no
	// knowledge_id for TaskInspector to match. Treat a matching durable ingest
	// row as the authoritative owner of a finalizing knowledge. Otherwise a
	// long-running/recovered Wiki batch can be falsely marked failed even
	// though its pending op is claimed and actively being processed.
	stuck, durableSkipped := h.filterOutDurableFinalizingOps(ctx, stuck)

	// Second-stage gate: a row can have a stale span heartbeat yet still
	// be perfectly healthy when its enrichment subtasks (summary /
	// question / graph / wiki) are merely backlogged behind a busy queue
	// — no worker has picked them up, so no span has been written since
	// post-process fanned them out. Killing such a row is the false-
	// positive users hit under heavy upload bursts. Drop any candidate
	// that still has a queued/active task referencing it; only rows with
	// nothing left in the queue are treated as genuinely orphaned.
	stuck, queueSkipped := h.filterOutQueued(ctx, stuck)

	if len(stuck) > 0 {
		if recovered, err := h.markStuckKnowledgeFailed(ctx, stuck, threshold, cutoff); err != nil {
			logger.Warnf(ctx, "[Housekeeping] knowledge sweep update failed: %v", err)
		} else if recovered > 0 {
			logger.Infof(ctx, "[Housekeeping] recovered %d stuck knowledge rows (threshold=%s)",
				recovered, threshold)
		}
	}
	if spanSkipped > 0 {
		// Visibility into "we considered killing N rows but their
		// span tree showed they're still progressing". Ops can grep
		// for this if they suspect housekeeping over- or under-fires.
		logger.Infof(ctx,
			"[Housekeeping] %d candidate(s) skipped — span heartbeat within threshold",
			spanSkipped)
	}
	if queueSkipped > 0 {
		// Visibility into "stale span heartbeat but tasks still queued"
		// — i.e. backpressure, not a stuck row. Persistent counts here
		// mean the queue is the bottleneck (raise the matching per-pool or
		// shared asynq concurrency, or document_process_timeout), not that
		// housekeeping misfires.
		logger.Infof(ctx,
			"[Housekeeping] %d candidate(s) skipped — tasks still queued (backpressure, not stuck)",
			queueSkipped)
	}
	if durableSkipped > 0 {
		logger.Infof(ctx,
			"[Housekeeping] %d candidate(s) skipped — durable Wiki operations still pending",
			durableSkipped)
	}

	// Sweep B: knowledge summary stuck. Summary is post-parse; threshold
	// is shorter because summary tasks are bounded by a single LLM call.
	// No span heartbeat exists for the summary stage (it lives in a
	// downstream asynq task), so we accept the original simple check.
	summaryCutoff := now.Add(-1 * time.Hour)
	resSummary := h.db.WithContext(ctx).Model(&types.Knowledge{}).
		Where("summary_status = ? AND updated_at < ?", types.SummaryStatusProcessing, summaryCutoff).
		Update("summary_status", types.SummaryStatusFailed)
	if resSummary.Error != nil {
		logger.Warnf(ctx, "[Housekeeping] summary sweep failed: %v", resSummary.Error)
	} else if resSummary.RowsAffected > 0 {
		logger.Infof(ctx, "[Housekeeping] recovered %d stuck summary rows", resSummary.RowsAffected)
	}
}

func (h *HousekeepingService) closeTerminalKnowledgeSpans(ctx context.Context, now time.Time) {
	terminalStatuses := []struct {
		knowledgeStatus string
		spanStatus      string
	}{
		{knowledgeStatus: types.ParseStatusCompleted, spanStatus: types.SpanStatusDone},
		{knowledgeStatus: types.ParseStatusFailed, spanStatus: types.SpanStatusCancelled},
	}
	for _, terminal := range terminalStatuses {
		knowledgeIDs := h.db.WithContext(ctx).Model(&types.Knowledge{}).
			Select("id").
			Where("parse_status = ?", terminal.knowledgeStatus)
		res := h.db.WithContext(ctx).Model(&types.KnowledgeProcessingSpan{}).
			Where("status = ?", types.SpanStatusRunning).
			Where("knowledge_id IN (?)", knowledgeIDs).
			Updates(map[string]interface{}{
				"status":      terminal.spanStatus,
				"finished_at": now,
			})
		if res.Error != nil {
			logger.Warnf(ctx,
				"[Housekeeping] terminal %s span reconciliation failed: %v",
				terminal.knowledgeStatus, res.Error)
			continue
		}
		if res.RowsAffected > 0 {
			logger.Infof(ctx,
				"[Housekeeping] closed %d running span(s) owned by %s knowledge",
				res.RowsAffected, terminal.knowledgeStatus)
		}
	}
}

// markStuckKnowledgeFailed applies the final state transition with guards that
// are re-evaluated by the database. In particular, a finalizing candidate may
// race its last subtask between the earlier scan and this write. Once its
// counter reaches zero it represents completed work and must not be overwritten
// with failed; the next sweep's zero-counter repair will promote it. A refreshed
// updated_at heartbeat likewise invalidates the stale candidate even when other
// subtasks remain outstanding.
func (h *HousekeepingService) markStuckKnowledgeFailed(
	ctx context.Context, candidates []types.Knowledge, threshold time.Duration, cutoff time.Time,
) (int64, error) {
	primaryIDs := make([]string, 0, len(candidates))
	finalizingIDs := make([]string, 0, len(candidates))
	for _, knowledge := range candidates {
		switch knowledge.ParseStatus {
		case types.ParseStatusPending, types.ParseStatusProcessing:
			primaryIDs = append(primaryIDs, knowledge.ID)
		case types.ParseStatusFinalizing:
			finalizingIDs = append(finalizingIDs, knowledge.ID)
		}
	}
	values := map[string]interface{}{
		"parse_status":           types.ParseStatusFailed,
		"error_message":          "task stuck in processing > " + threshold.String() + ", recovered by housekeeping",
		"pending_subtasks_count": 0,
	}
	var recovered int64
	if len(primaryIDs) > 0 {
		updatePrimary := func(guardSpanHeartbeat bool) *gorm.DB {
			query := h.db.WithContext(ctx).Model(&types.Knowledge{}).
				Where("id IN ? AND parse_status IN ?", primaryIDs,
					[]string{types.ParseStatusPending, types.ParseStatusProcessing}).
				Where("updated_at < ?", cutoff)
			if guardSpanHeartbeat {
				query = withoutRecentSpanHeartbeat(query, cutoff)
			}
			return query.Updates(values)
		}
		res := updatePrimary(true)
		if res.Error != nil && isMissingTable(res.Error, "knowledge_processing_spans") {
			logger.Warnf(ctx,
				"[Housekeeping] knowledge_processing_spans table unavailable; applying compatibility recovery without span guard")
			res = updatePrimary(false)
		}
		if res.Error != nil {
			return recovered, res.Error
		}
		recovered += res.RowsAffected
	}
	if len(finalizingIDs) > 0 {
		now := time.Now()
		updateFinalizing := func(guardSpanHeartbeat, guardDurableOwner bool) *gorm.DB {
			query := h.db.WithContext(ctx).Model(&types.Knowledge{}).
				Where("id IN ? AND parse_status = ? AND pending_subtasks_count > 0",
					finalizingIDs, types.ParseStatusFinalizing).
				Where("updated_at < ?", cutoff)
			if guardSpanHeartbeat {
				query = withoutRecentSpanHeartbeat(query, cutoff)
			}
			if guardDurableOwner {
				query = query.Where(`NOT EXISTS (
				SELECT 1 FROM task_pending_ops pending
				WHERE pending.tenant_id = knowledges.tenant_id
					AND pending.task_type = ?
					AND pending.scope = ?
					AND pending.scope_id = knowledges.knowledge_base_id
					AND pending.op = ?
					AND pending.dedup_key = knowledges.id
					AND (pending.enqueued_at > ? OR pending.claimed_at > ?)
				)`, types.TypeWikiIngest, types.TaskScopeKnowledgeBase, WikiOpIngest,
					now.Add(-durableWikiOwnershipMaxAge), now.Add(-wikiClaimStaleAfter))
			}
			return query.Updates(values)
		}

		guardSpanHeartbeat := true
		guardDurableOwner := true
		var res *gorm.DB
		for {
			res = updateFinalizing(guardSpanHeartbeat, guardDurableOwner)
			if res.Error == nil {
				break
			}
			if guardSpanHeartbeat && isMissingTable(res.Error, "knowledge_processing_spans") {
				logger.Warnf(ctx,
					"[Housekeeping] knowledge_processing_spans table unavailable; applying compatibility recovery without span guard")
				guardSpanHeartbeat = false
				continue
			}
			if guardDurableOwner && isMissingTable(res.Error, "task_pending_ops") {
				// task_pending_ops was introduced after knowledges. During a
				// rolling upgrade (and in Lite databases created by old versions),
				// preserve the pre-durable-owner recovery behavior only for this
				// explicit schema-compatibility case. Other database errors remain
				// fatal so a transient owner-query failure cannot reopen the race.
				logger.Warnf(ctx,
					"[Housekeeping] task_pending_ops table unavailable; applying compatibility recovery without durable-owner guard")
				guardDurableOwner = false
				continue
			}
			break
		}
		if res.Error != nil {
			return recovered, res.Error
		}
		recovered += res.RowsAffected
	}
	return recovered, nil
}

func withoutRecentSpanHeartbeat(query *gorm.DB, cutoff time.Time) *gorm.DB {
	return query.Where(`NOT EXISTS (
		SELECT 1 FROM knowledge_processing_spans recent_span
		WHERE recent_span.knowledge_id = knowledges.id
			AND recent_span.updated_at >= ?
	)`, cutoff)
}

func isMissingTable(err error, table string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, strings.ToLower(table)) {
		return false
	}
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "undefined table")
}

// filterOutDurableFinalizingOps removes candidates that are still owned by a
// durable wiki:ingest row. Redis inspection cannot make this association: the
// Wiki trigger payload is knowledge-base scoped, while dedup_key in
// task_pending_ops is the knowledge ID. Match all three scope dimensions so an
// identical knowledge ID in another tenant/KB cannot protect the wrong row.
//
// A durable op is only authoritative for finalizing rows and only within the
// bounded ownership window, while its claim is fresh, or while the KB-scoped
// Wiki trigger is verifiably alive. The last condition preserves legitimate
// >24h bulk-import backlogs without allowing a permanently triggerless row to
// mask failure forever. Pending/processing rows belong to the primary parse
// pipeline and must continue through the normal span + TaskInspector checks.
// On query error we retain the candidates, matching the housekeeping sweep's
// existing fail-safe recovery direction.
func (h *HousekeepingService) filterOutDurableFinalizingOps(
	ctx context.Context, candidates []types.Knowledge,
) (kept []types.Knowledge, skipped int) {
	if len(candidates) == 0 {
		return candidates, 0
	}

	ids := make([]string, 0, len(candidates))
	tenantSet := make(map[uint64]struct{})
	scopeSet := make(map[string]struct{})
	for _, knowledge := range candidates {
		if knowledge.ParseStatus == types.ParseStatusFinalizing {
			ids = append(ids, knowledge.ID)
			tenantSet[knowledge.TenantID] = struct{}{}
			scopeSet[knowledge.KnowledgeBaseID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return candidates, 0
	}

	type durableOwner struct {
		TenantID   uint64     `gorm:"column:tenant_id"`
		ScopeID    string     `gorm:"column:scope_id"`
		DedupKey   string     `gorm:"column:dedup_key"`
		EnqueuedAt time.Time  `gorm:"column:enqueued_at"`
		ClaimedAt  *time.Time `gorm:"column:claimed_at"`
	}
	tenantIDs := make([]uint64, 0, len(tenantSet))
	for tenantID := range tenantSet {
		tenantIDs = append(tenantIDs, tenantID)
	}
	scopeIDs := make([]string, 0, len(scopeSet))
	for scopeID := range scopeSet {
		scopeIDs = append(scopeIDs, scopeID)
	}
	var owners []durableOwner
	now := time.Now()
	if err := h.db.WithContext(ctx).
		Model(&types.TaskPendingOp{}).
		Select("tenant_id, scope_id, dedup_key, enqueued_at, claimed_at").
		Where("task_type = ? AND scope = ? AND op = ? AND tenant_id IN ? AND scope_id IN ? AND dedup_key IN ?",
			types.TypeWikiIngest, types.TaskScopeKnowledgeBase, WikiOpIngest,
			tenantIDs, scopeIDs, ids).
		Find(&owners).Error; err != nil {
		logger.Warnf(ctx,
			"[Housekeeping] durable Wiki ownership query failed: %v (will fail safe and recover candidates)", err)
		return candidates, 0
	}

	type ownerKey struct {
		tenantID        uint64
		knowledgeBaseID string
		knowledgeID     string
	}
	owned := make(map[ownerKey]struct{}, len(owners))
	kbLiveness := make(map[string]bool)
	kbLivenessChecked := make(map[string]struct{})
	kbInspector, canInspectKB := h.inspector.(interfaces.KnowledgeBaseTaskQueueInspector)
	for _, owner := range owners {
		fresh := owner.EnqueuedAt.After(now.Add(-durableWikiOwnershipMaxAge)) ||
			(owner.ClaimedAt != nil && owner.ClaimedAt.After(now.Add(-wikiClaimStaleAfter)))
		if !fresh && canInspectKB {
			if _, checked := kbLivenessChecked[owner.ScopeID]; !checked {
				live, err := kbInspector.HasQueuedTasksForKnowledgeBase(ctx, owner.ScopeID)
				if err != nil {
					logger.Warnf(ctx,
						"[Housekeeping] Wiki trigger probe failed for KB %s: %v", owner.ScopeID, err)
				}
				kbLiveness[owner.ScopeID] = err == nil && live
				kbLivenessChecked[owner.ScopeID] = struct{}{}
			}
			fresh = kbLiveness[owner.ScopeID]
		}
		if fresh {
			owned[ownerKey{owner.TenantID, owner.ScopeID, owner.DedupKey}] = struct{}{}
		}
	}

	out := candidates[:0]
	for _, knowledge := range candidates {
		if knowledge.ParseStatus == types.ParseStatusFinalizing {
			key := ownerKey{knowledge.TenantID, knowledge.KnowledgeBaseID, knowledge.ID}
			if _, ok := owned[key]; ok {
				skipped++
				continue
			}
		}
		out = append(out, knowledge)
	}
	return out, skipped
}

// filterByLastSpanActivity returns the subset of candidates whose most
// recent span row predates `cutoff` — i.e. genuinely stuck. Candidates
// with no span rows at all also pass through (they're lite-mode or
// pre-instrumentation tasks; the simple updated_at check already proved
// them stuck and we have no heartbeat to override that).
func (h *HousekeepingService) filterByLastSpanActivity(ctx context.Context, candidates []types.Knowledge, cutoff time.Time) []types.Knowledge {
	if len(candidates) == 0 {
		return candidates
	}
	ids := make([]string, 0, len(candidates))
	for _, k := range candidates {
		ids = append(ids, k.ID)
	}

	// We scan MAX(updated_at) as string then parse client-side. That
	// dodges the SQLite driver's well-known refusal to auto-convert
	// aggregate datetime values into time.Time on its own — Postgres
	// happily round-trips, but the same query shape must work in
	// Lite mode too. Since we only compare against a cutoff, the
	// parse layer below tries the formats both Postgres and SQLite
	// emit and takes the first that parses.
	type spanHeartbeat struct {
		KnowledgeID string `gorm:"column:knowledge_id"`
		LastSeen    string `gorm:"column:last_seen"`
	}
	var beats []spanHeartbeat
	err := h.db.WithContext(ctx).
		Table("knowledge_processing_spans").
		Select("knowledge_id, MAX(updated_at) AS last_seen").
		Where("knowledge_id IN ?", ids).
		Group("knowledge_id").
		Find(&beats).Error
	if err != nil {
		// On query failure, fail safe — assume nothing has a
		// heartbeat (so all candidates are "stuck"). This matches
		// the previous-version behaviour and never under-recovers.
		logger.Warnf(ctx, "[Housekeeping] span heartbeat query failed: %v (will fail safe and recover all candidates)", err)
		return candidates
	}
	heartbeat := make(map[string]time.Time, len(beats))
	for _, b := range beats {
		if t, ok := parseHeartbeatTime(b.LastSeen); ok {
			heartbeat[b.KnowledgeID] = t
		}
	}

	out := candidates[:0]
	for _, k := range candidates {
		if last, ok := heartbeat[k.ID]; ok && last.After(cutoff) {
			// Active span heartbeat — leave alone.
			continue
		}
		out = append(out, k)
	}
	return out
}

// filterOutQueued returns the subset of candidates that have NO task left
// in the queue backend, plus a count of how many were dropped because a
// task still references them. A dropped candidate is "backlogged, not
// orphaned" — its enrichment subtasks are waiting for a worker, so the
// missing span heartbeat is expected and recovering it would be a false
// positive. When no inspector is wired (nil) the gate is a pass-through
// so behaviour matches the pre-existing span-only sweep. On inspector
// error we fail safe by KEEPING the candidate as stuck (recover it),
// matching the span heartbeat query's fail-safe direction.
func (h *HousekeepingService) filterOutQueued(
	ctx context.Context, candidates []types.Knowledge,
) (kept []types.Knowledge, skipped int) {
	if h.inspector == nil || len(candidates) == 0 {
		return candidates, 0
	}
	out := candidates[:0]
	for _, k := range candidates {
		queued, err := h.inspector.HasQueuedTasksForKnowledge(ctx, k.ID)
		if err != nil {
			logger.Warnf(ctx,
				"[Housekeeping] queue probe failed for %s: %v (will fail safe and treat as stuck)", k.ID, err)
			out = append(out, k)
			continue
		}
		if queued {
			skipped++
			continue
		}
		out = append(out, k)
	}
	return out, skipped
}

// parseHeartbeatTime accepts the timestamp formats Postgres and SQLite
// emit for a TIMESTAMP column read back through MAX(). Returns false if
// none parse — the caller treats unparseable rows as "no heartbeat",
// which fails safe (the row gets recovered as stuck rather than
// silently preserved).
func parseHeartbeatTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// staleThreshold returns how long a "processing" row may sit untouched
// before housekeeping treats it as orphaned. The floor is 1 hour so that a
// genuinely slow large-PDF parse cannot be killed mid-flight; the ceiling
// scales with the operator-configured DocumentProcessTimeout plus 10 minute
// buffer to absorb scheduling jitter.
func (h *HousekeepingService) staleThreshold() time.Duration {
	base := 1 * time.Hour
	if h.cfg != nil && h.cfg.KnowledgeBase != nil && h.cfg.KnowledgeBase.DocumentProcessTimeout > base {
		base = h.cfg.KnowledgeBase.DocumentProcessTimeout
	}
	return base + 10*time.Minute
}

func housekeepingEnabled() bool {
	// Default-on: missing/empty env enables the sweep. Operators must
	// explicitly set "false" to opt out, matching the plan's commitment
	// that no env change is required for the safety net to engage.
	v := strings.TrimSpace(os.Getenv("WEKNORA_HOUSEKEEPING_ENABLED"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}
