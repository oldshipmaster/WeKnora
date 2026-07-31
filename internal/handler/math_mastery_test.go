package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mathMasteryServiceHandlerStub struct {
	tree       *types.MathMasteryTree
	seeded     bool
	assessment *types.MathMasteryAssessment
	questions  []types.MathQuestion
}

func (s *mathMasteryServiceHandlerStub) SeedCurriculum(context.Context, string, []types.MathCurriculumNode, []types.MathCurriculumEdge) error {
	s.seeded = true
	return nil
}
func (s *mathMasteryServiceHandlerStub) GetTree(context.Context, string) (*types.MathMasteryTree, error) {
	return s.tree, nil
}
func (s *mathMasteryServiceHandlerStub) UpsertSources(context.Context, string, []types.MathSourceBinding) error {
	return nil
}
func (s *mathMasteryServiceHandlerStub) UpsertQuestions(_ context.Context, _ string, questions []types.MathQuestion, _ []types.MathQuestionNode) error {
	s.questions = append([]types.MathQuestion(nil), questions...)
	return nil
}
func (s *mathMasteryServiceHandlerStub) ListQuestions(context.Context, string, string, int) ([]types.MathQuestion, error) {
	return append([]types.MathQuestion(nil), s.questions...), nil
}
func (s *mathMasteryServiceHandlerStub) StartAttempt(context.Context, string, string, types.JSON) (*types.MathDiagnosticAttempt, error) {
	return &types.MathDiagnosticAttempt{ID: "attempt-1"}, nil
}
func (s *mathMasteryServiceHandlerStub) SubmitResponse(context.Context, string, types.MathDiagnosticResponse) (*types.MathMasteryAssessment, error) {
	return s.assessment, nil
}

func newMathMasteryHandlerRouter(service *mathMasteryServiceHandlerStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(42))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	handler := NewMathMasteryHandler(service)
	router.GET("/knowledge-bases/:id/math-mastery/tree", handler.GetTree)
	router.GET("/knowledge-bases/:id/math-mastery/questions", handler.ListQuestions)
	router.POST("/knowledge-bases/:id/math-mastery/seed", handler.SeedCurriculum)
	router.PUT("/knowledge-bases/:id/math-mastery/questions", handler.UpsertQuestions)
	router.POST("/knowledge-bases/:id/math-mastery/responses", handler.SubmitResponse)
	return router
}

func TestMathMasteryHandlerReturnsTree(t *testing.T) {
	service := &mathMasteryServiceHandlerStub{tree: &types.MathMasteryTree{
		Overview: types.MathMasteryOverview{TotalNodes: 1},
		Nodes:    []types.MathMasteryNodeView{{MathCurriculumNode: types.MathCurriculumNode{ID: "number-5"}}},
	}}
	router := newMathMasteryHandlerRouter(service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/knowledge-bases/kb-1/math-mastery/tree", nil))

	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Success bool                  `json:"success"`
		Data    types.MathMasteryTree `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 1, body.Data.Overview.TotalNodes)
}

func TestMathMasteryHandlerRejectsInvalidSeedPayload(t *testing.T) {
	service := &mathMasteryServiceHandlerStub{}
	router := newMathMasteryHandlerRouter(service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/knowledge-bases/kb-1/math-mastery/seed", bytes.NewBufferString(`{"nodes":`)))

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.False(t, service.seeded)
}

func TestMathMasteryHandlerReturnsUpdatedAssessment(t *testing.T) {
	service := &mathMasteryServiceHandlerStub{assessment: &types.MathMasteryAssessment{State: types.MathMasteryDeveloping, EvidenceCount: 1}}
	router := newMathMasteryHandlerRouter(service)
	payload := `{"attempt_id":"attempt-1","question_id":"q1","node_id":"number-5","question_type":"basic","correct":true}`

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/knowledge-bases/kb-1/math-mastery/responses", bytes.NewBufferString(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"state":"developing"`)
}

func TestMathMasteryHandlerListsImportedQuestions(t *testing.T) {
	service := &mathMasteryServiceHandlerStub{questions: []types.MathQuestion{{
		ID: "q1", SourceBindingID: "exam-1", QuestionLocator: "PDF 第 3 页", QuestionType: "calculation",
		ScoringRule: types.JSON(`{"prompt":"计算题"}`),
	}}}
	router := newMathMasteryHandlerRouter(service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/knowledge-bases/kb-1/math-mastery/questions?node_id=g1s1-number-5&limit=3", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"question_locator":"PDF 第 3 页"`)
}
