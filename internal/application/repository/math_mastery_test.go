package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMathMasteryTestRepository(t *testing.T) *mathMasteryRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.MathCurriculumNode{},
		&types.MathCurriculumEdge{},
		&types.MathSourceBinding{},
		&types.MathQuestion{},
		&types.MathQuestionNode{},
		&types.MathDiagnosticAttempt{},
		&types.MathDiagnosticResponse{},
	))
	return &mathMasteryRepository{db: db}
}

func TestMathMasteryRepositoryUpsertsAndOrdersCurriculum(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	nodes := []types.MathCurriculumNode{
		{ID: "later", NodeType: "concept", Grade: 2, Term: 1, Domain: "number", Title: "later", SortOrder: 1},
		{ID: "first", NodeType: "concept", Grade: 1, Term: 1, Domain: "number", Title: "old title", SortOrder: 1},
	}
	edges := []types.MathCurriculumEdge{{ID: "edge-1", FromNodeID: "first", ToNodeID: "later", RelationType: "prerequisite", Strength: 1}}

	require.NoError(t, repo.UpsertCurriculum(ctx, 7, "kb-1", nodes, edges))
	nodes[1].Title = "new title"
	require.NoError(t, repo.UpsertCurriculum(ctx, 7, "kb-1", nodes[1:], nil))

	gotNodes, gotEdges, err := repo.ListCurriculum(ctx, 7, "kb-1")
	require.NoError(t, err)
	require.Len(t, gotNodes, 2)
	require.Equal(t, "first", gotNodes[0].ID)
	require.Equal(t, "new title", gotNodes[0].Title)
	require.Equal(t, "later", gotNodes[1].ID)
	require.Len(t, gotEdges, 1)
	require.Equal(t, uint64(7), gotNodes[0].TenantID)
	require.Equal(t, "kb-1", gotNodes[0].KnowledgeBaseID)
	require.JSONEq(t, `{}`, string(gotNodes[0].Metadata))
}

func TestMathMasteryRepositoryUpdatesSourceStatusIdempotently(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	source := &types.MathSourceBinding{
		ID: "source-1", TargetID: "rj-g1-s1-textbook", SourceType: "textbook",
		Status: "found", Grade: 1, Term: 1, Edition: "人教版",
	}
	require.NoError(t, repo.UpsertSource(ctx, 8, "kb-2", source))
	source.Status = "ready"
	source.KnowledgeID = "knowledge-1"
	require.NoError(t, repo.UpsertSource(ctx, 8, "kb-2", source))

	sources, err := repo.ListSources(ctx, 8, "kb-2")
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, "ready", sources[0].Status)
	require.Equal(t, "knowledge-1", sources[0].KnowledgeID)
}

func TestMathMasteryRepositoryMigratesFallbackSourceIDWhenMaterialBecomesAvailable(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	placeholder := &types.MathSourceBinding{
		ID: "rj-g1-s2-exam", TargetID: "rj-g1-s2-exam", SourceType: "exam",
		Status: "missing", Grade: 1, Term: 2, Edition: "人教版",
	}
	require.NoError(t, repo.UpsertSource(ctx, 8, "kb-2", placeholder))
	require.NoError(t, repo.UpsertQuestions(ctx, 8, "kb-2", []types.MathQuestion{{
		ID: "q-1", SourceBindingID: placeholder.ID, QuestionLocator: "PDF 第 1 页", QuestionType: "calculation",
	}}, []types.MathQuestionNode{{QuestionID: "q-1", NodeID: "g1-add", IsPrimary: true, Confidence: 0.9}}))

	material := &types.MathSourceBinding{
		ID: "material-abc", TargetID: placeholder.TargetID, SourceType: "exam", Status: "processing",
		KnowledgeID: "knowledge-1", Grade: 1, Term: 2, Edition: "人教版",
	}
	require.NoError(t, repo.UpsertSource(ctx, 8, "kb-2", material))

	sources, err := repo.ListSources(ctx, 8, "kb-2")
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, material.ID, sources[0].ID)
	require.Equal(t, "knowledge-1", sources[0].KnowledgeID)
	var question types.MathQuestion
	require.NoError(t, repo.db.Where("tenant_id = ? AND knowledge_base_id = ? AND id = ?", 8, "kb-2", "q-1").First(&question).Error)
	require.Equal(t, material.ID, question.SourceBindingID)
}

func TestMathMasteryRepositoryCreatesTraceableDiagnosticEvidence(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	attempt := &types.MathDiagnosticAttempt{ID: "session-1", ProfileID: "local-child", Scope: types.JSON(`{"grade":1}`), Status: "completed"}
	require.NoError(t, repo.CreateAttempt(ctx, 9, "kb-3", attempt))
	require.NoError(t, repo.AddResponse(ctx, 9, "kb-3", &types.MathDiagnosticResponse{
		ID: "response-1", AttemptID: "session-1", QuestionID: "q1", NodeID: "number-5",
		QuestionType: "basic", Correct: true, Weight: 1,
	}))

	evidence, err := repo.ListEvidence(ctx, 9, "kb-3", "number-5")
	require.NoError(t, err)
	require.Equal(t, []types.MathMasteryEvidence{{
		QuestionID: "q1", QuestionType: "basic", SessionID: "session-1", Correct: true, Weight: 1,
	}}, evidence)
}

func TestMathMasteryRepositoryUpsertsAndListsQuestionsByCurriculumNode(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	questions := []types.MathQuestion{
		{ID: "q-hard", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 12 页 · 第 8 题", QuestionType: "application", Difficulty: 0.8, ScoringRule: types.JSON(`{"prompt":"应用题"}`)},
		{ID: "q-basic", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 3 页 · 第 2 题", QuestionType: "calculation", Difficulty: 0.3, ScoringRule: types.JSON(`{"prompt":"计算题"}`)},
	}
	links := []types.MathQuestionNode{
		{QuestionID: "q-hard", NodeID: "g1s1-number-5", IsPrimary: true, Confidence: 0.9},
		{QuestionID: "q-basic", NodeID: "g1s1-number-5", IsPrimary: true, Confidence: 0.95},
	}

	require.NoError(t, repo.UpsertQuestions(ctx, 9, "kb-3", questions, links))
	got, err := repo.ListQuestions(ctx, 9, "kb-3", "g1s1-number-5", 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "q-basic", got[0].ID)
	require.JSONEq(t, `{"prompt":"计算题"}`, string(got[0].ScoringRule))
}

func TestMathMasteryRepositoryCountsImportedQuestionsByCurriculumNode(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	questions := []types.MathQuestion{
		{ID: "q-1", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 1 页", QuestionType: "calculation"},
		{ID: "q-2", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 2 页", QuestionType: "calculation"},
		{ID: "q-3", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 3 页", QuestionType: "application"},
	}
	links := []types.MathQuestionNode{
		{QuestionID: "q-1", NodeID: "number-10", IsPrimary: true, Confidence: 0.9},
		{QuestionID: "q-2", NodeID: "number-10", IsPrimary: true, Confidence: 0.9},
		{QuestionID: "q-3", NodeID: "number-20", IsPrimary: true, Confidence: 0.9},
		{QuestionID: "q-1", NodeID: "number-20", IsPrimary: false, Confidence: 0.8},
	}
	require.NoError(t, repo.UpsertQuestions(ctx, 9, "kb-3", questions, links))
	require.NoError(t, repo.UpsertQuestions(ctx, 10, "kb-other", questions[:1], []types.MathQuestionNode{{
		QuestionID: "q-1", NodeID: "number-10", IsPrimary: true, Confidence: 0.9,
	}}))

	counts, total, err := repo.ListQuestionCounts(ctx, 9, "kb-3")
	require.NoError(t, err)
	require.Equal(t, map[string]int{"number-10": 2, "number-20": 2}, counts)
	require.Equal(t, 3, total)
}

func TestMathMasteryRepositoryQuestionCountsIgnoreOrphanLinks(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.db.Create(&types.MathQuestionNode{
		TenantID: 9, KnowledgeBaseID: "kb-3", QuestionID: "missing-question", NodeID: "number-10",
		IsPrimary: true, Confidence: 0.9,
	}).Error)

	counts, total, err := repo.ListQuestionCounts(ctx, 9, "kb-3")
	require.NoError(t, err)
	require.Empty(t, counts)
	require.Zero(t, total)
}

func TestMathMasteryRepositoryKeepsTenantsAndKnowledgeBasesIsolated(t *testing.T) {
	repo := newMathMasteryTestRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.UpsertCurriculum(ctx, 1, "kb-a", []types.MathCurriculumNode{{
		ID: "shared-id", NodeType: "concept", Grade: 1, Term: 1, Domain: "number", Title: "tenant one",
	}}, nil))
	require.NoError(t, repo.UpsertCurriculum(ctx, 2, "kb-b", []types.MathCurriculumNode{{
		ID: "shared-id", NodeType: "concept", Grade: 1, Term: 1, Domain: "number", Title: "tenant two",
	}}, nil))

	nodes, _, err := repo.ListCurriculum(ctx, 1, "kb-a")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "tenant one", nodes[0].Title)
}
