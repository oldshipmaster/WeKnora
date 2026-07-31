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
	CreateAttempt(ctx context.Context, tenantID uint64, kbID string, attempt *types.MathDiagnosticAttempt) error
	AddResponse(ctx context.Context, tenantID uint64, kbID string, response *types.MathDiagnosticResponse) error
	ListEvidence(ctx context.Context, tenantID uint64, kbID, nodeID string) ([]types.MathMasteryEvidence, error)
}
