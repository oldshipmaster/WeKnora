package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type mathMasteryRepoStub struct {
	nodes          []types.MathCurriculumNode
	edges          []types.MathCurriculumEdge
	sources        []types.MathSourceBinding
	evidence       map[string][]types.MathMasteryEvidence
	attempts       []types.MathDiagnosticAttempt
	responses      []types.MathDiagnosticResponse
	questions      []types.MathQuestion
	links          []types.MathQuestionNode
	questionCounts map[string]int
	questionTotal  int
}

func (r *mathMasteryRepoStub) UpsertCurriculum(_ context.Context, tenantID uint64, kbID string, nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) error {
	for index := range nodes {
		nodes[index].TenantID = tenantID
		nodes[index].KnowledgeBaseID = kbID
	}
	for index := range edges {
		edges[index].TenantID = tenantID
		edges[index].KnowledgeBaseID = kbID
	}
	r.nodes = append([]types.MathCurriculumNode(nil), nodes...)
	r.edges = append([]types.MathCurriculumEdge(nil), edges...)
	return nil
}

func (r *mathMasteryRepoStub) ListCurriculum(context.Context, uint64, string) ([]types.MathCurriculumNode, []types.MathCurriculumEdge, error) {
	return append([]types.MathCurriculumNode(nil), r.nodes...), append([]types.MathCurriculumEdge(nil), r.edges...), nil
}

func (r *mathMasteryRepoStub) UpsertSource(_ context.Context, tenantID uint64, kbID string, source *types.MathSourceBinding) error {
	source.TenantID = tenantID
	source.KnowledgeBaseID = kbID
	r.sources = append(r.sources, *source)
	return nil
}

func (r *mathMasteryRepoStub) ListSources(context.Context, uint64, string) ([]types.MathSourceBinding, error) {
	return append([]types.MathSourceBinding(nil), r.sources...), nil
}

func (r *mathMasteryRepoStub) UpsertQuestions(_ context.Context, tenantID uint64, kbID string, questions []types.MathQuestion, links []types.MathQuestionNode) error {
	for index := range questions {
		questions[index].TenantID = tenantID
		questions[index].KnowledgeBaseID = kbID
	}
	r.questions = append([]types.MathQuestion(nil), questions...)
	r.links = append([]types.MathQuestionNode(nil), links...)
	return nil
}

func (r *mathMasteryRepoStub) ListQuestions(context.Context, uint64, string, string, int) ([]types.MathQuestion, error) {
	return append([]types.MathQuestion(nil), r.questions...), nil
}

func (r *mathMasteryRepoStub) ListQuestionCounts(context.Context, uint64, string) (map[string]int, int, error) {
	result := make(map[string]int, len(r.questionCounts))
	for nodeID, count := range r.questionCounts {
		result[nodeID] = count
	}
	return result, r.questionTotal, nil
}

func (r *mathMasteryRepoStub) CreateAttempt(_ context.Context, tenantID uint64, kbID string, attempt *types.MathDiagnosticAttempt) error {
	attempt.TenantID = tenantID
	attempt.KnowledgeBaseID = kbID
	r.attempts = append(r.attempts, *attempt)
	return nil
}

func (r *mathMasteryRepoStub) AddResponse(_ context.Context, tenantID uint64, kbID string, response *types.MathDiagnosticResponse) error {
	response.TenantID = tenantID
	response.KnowledgeBaseID = kbID
	r.responses = append(r.responses, *response)
	r.evidence[response.NodeID] = append(r.evidence[response.NodeID], types.MathMasteryEvidence{
		QuestionID: response.QuestionID, QuestionType: response.QuestionType,
		SessionID: response.AttemptID, Correct: response.Correct, Weight: response.Weight,
	})
	return nil
}

func (r *mathMasteryRepoStub) ListEvidence(_ context.Context, _ uint64, _, nodeID string) ([]types.MathMasteryEvidence, error) {
	return append([]types.MathMasteryEvidence(nil), r.evidence[nodeID]...), nil
}

func masteryTestContext() context.Context {
	return context.WithValue(context.Background(), types.TenantIDContextKey, uint64(42))
}

func TestMathMasteryServiceRejectsCyclicSeed(t *testing.T) {
	repo := &mathMasteryRepoStub{evidence: map[string][]types.MathMasteryEvidence{}}
	service := NewMathMasteryService(repo)
	nodes := []types.MathCurriculumNode{{ID: "a", Grade: 1, Term: 1}, {ID: "b", Grade: 1, Term: 1}}
	edges := []types.MathCurriculumEdge{
		{FromNodeID: "a", ToNodeID: "b", RelationType: "prerequisite"},
		{FromNodeID: "b", ToNodeID: "a", RelationType: "prerequisite"},
	}

	err := service.SeedCurriculum(masteryTestContext(), "kb-1", nodes, edges)
	require.Error(t, err)
	require.Empty(t, repo.nodes)
}

func TestMathMasteryServiceExplainsBlockedNodesAndOverview(t *testing.T) {
	repo := &mathMasteryRepoStub{
		nodes: []types.MathCurriculumNode{
			{ID: "number-5", Grade: 1, Term: 1, Domain: "number", Title: "5以内数"},
			{ID: "number-10", Grade: 1, Term: 1, Domain: "operations", Title: "10以内加减"},
		},
		edges: []types.MathCurriculumEdge{{FromNodeID: "number-5", ToNodeID: "number-10", RelationType: "prerequisite"}},
		evidence: map[string][]types.MathMasteryEvidence{
			"number-5": {{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: false}},
		},
		questionCounts: map[string]int{"number-5": 1, "number-10": 1},
		questionTotal:  1,
	}
	service := NewMathMasteryService(repo)

	tree, err := service.GetTree(masteryTestContext(), "kb-1")
	require.NoError(t, err)
	require.Len(t, tree.Nodes, 2)
	require.Equal(t, types.MathMasteryWeak, tree.Nodes[0].Assessment.State)
	require.Equal(t, []string{"number-5"}, tree.Nodes[1].BlockedBy)
	require.Equal(t, 2, tree.Overview.TotalNodes)
	require.Equal(t, 1, tree.Overview.WeakNodes)
	require.Equal(t, 1, tree.Overview.UntestedNodes)
	require.Equal(t, 1, tree.Nodes[0].QuestionCount)
	require.Equal(t, 1, tree.Nodes[1].QuestionCount)
	require.Equal(t, 1, tree.Overview.DiagnosticQuestions)
	require.Equal(t, 2, tree.Overview.NodesWithQuestions)
}

func TestMathMasteryServiceSubmitsResponseAndReturnsUpdatedAssessment(t *testing.T) {
	repo := &mathMasteryRepoStub{
		nodes:    []types.MathCurriculumNode{{ID: "number-5", Grade: 1, Term: 1, Domain: "number", Title: "5以内数"}},
		evidence: map[string][]types.MathMasteryEvidence{},
	}
	service := NewMathMasteryService(repo)

	attempt, err := service.StartAttempt(masteryTestContext(), "kb-1", "local-child", nil)
	require.NoError(t, err)
	require.NotEmpty(t, attempt.ID)
	assessment, err := service.SubmitResponse(masteryTestContext(), "kb-1", types.MathDiagnosticResponse{
		AttemptID: attempt.ID, QuestionID: "q1", NodeID: "number-5", QuestionType: "basic", Correct: true,
	})
	require.NoError(t, err)
	require.Equal(t, types.MathMasteryDeveloping, assessment.State)
	require.Len(t, repo.responses, 1)
}

func TestMathMasteryServiceImportsTraceableQuestions(t *testing.T) {
	repo := &mathMasteryRepoStub{evidence: map[string][]types.MathMasteryEvidence{}}
	service := NewMathMasteryService(repo)
	questions := []types.MathQuestion{{
		ID: "q1", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 3 页 · 第 2 题",
		QuestionType: "calculation", Difficulty: 0.3, ExtractionConfidence: 0.9,
	}}
	links := []types.MathQuestionNode{{QuestionID: "q1", NodeID: "g1s1-number-5", IsPrimary: true, Confidence: 0.9}}

	require.NoError(t, service.UpsertQuestions(masteryTestContext(), "kb-1", questions, links))
	require.Len(t, repo.questions, 1)
	require.Equal(t, uint64(42), repo.questions[0].TenantID)
	listed, err := service.ListQuestions(masteryTestContext(), "kb-1", "g1s1-number-5", 5)
	require.NoError(t, err)
	require.Equal(t, "q1", listed[0].ID)
}
