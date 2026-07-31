package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type MathMasteryRepository interface {
	UpsertCurriculum(ctx context.Context, tenantID uint64, kbID string, nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) error
	ListCurriculum(ctx context.Context, tenantID uint64, kbID string) ([]types.MathCurriculumNode, []types.MathCurriculumEdge, error)
	UpsertSource(ctx context.Context, tenantID uint64, kbID string, source *types.MathSourceBinding) error
	ListSources(ctx context.Context, tenantID uint64, kbID string) ([]types.MathSourceBinding, error)
	UpsertQuestions(ctx context.Context, tenantID uint64, kbID string, questions []types.MathQuestion, links []types.MathQuestionNode) error
	ListQuestions(ctx context.Context, tenantID uint64, kbID, nodeID string, limit int) ([]types.MathQuestion, error)
	CreateAttempt(ctx context.Context, tenantID uint64, kbID string, attempt *types.MathDiagnosticAttempt) error
	AddResponse(ctx context.Context, tenantID uint64, kbID string, response *types.MathDiagnosticResponse) error
	ListEvidence(ctx context.Context, tenantID uint64, kbID, nodeID string) ([]types.MathMasteryEvidence, error)
}

type MathMasteryService interface {
	SeedCurriculum(ctx context.Context, kbID string, nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) error
	GetTree(ctx context.Context, kbID string) (*types.MathMasteryTree, error)
	UpsertSources(ctx context.Context, kbID string, sources []types.MathSourceBinding) error
	UpsertQuestions(ctx context.Context, kbID string, questions []types.MathQuestion, links []types.MathQuestionNode) error
	ListQuestions(ctx context.Context, kbID, nodeID string, limit int) ([]types.MathQuestion, error)
	StartAttempt(ctx context.Context, kbID, profileID string, scope types.JSON) (*types.MathDiagnosticAttempt, error)
	SubmitResponse(ctx context.Context, kbID string, response types.MathDiagnosticResponse) (*types.MathMasteryAssessment, error)
}
