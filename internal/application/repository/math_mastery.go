package repository

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type mathMasteryRepository struct {
	db *gorm.DB
}

func NewMathMasteryRepository(db *gorm.DB) interfaces.MathMasteryRepository {
	return &mathMasteryRepository{db: db}
}

func (r *mathMasteryRepository) UpsertCurriculum(ctx context.Context, tenantID uint64, kbID string, nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index := range nodes {
			nodes[index].TenantID = tenantID
			nodes[index].KnowledgeBaseID = kbID
			if len(nodes[index].Metadata) == 0 {
				nodes[index].Metadata = types.JSON(`{}`)
			}
			if err := tx.Clauses(scopeUpsertClause()).Create(&nodes[index]).Error; err != nil {
				return fmt.Errorf("upsert curriculum node %q: %w", nodes[index].ID, err)
			}
		}
		for index := range edges {
			edges[index].TenantID = tenantID
			edges[index].KnowledgeBaseID = kbID
			if edges[index].ID == "" {
				edges[index].ID = edges[index].FromNodeID + "--" + edges[index].ToNodeID + "--" + edges[index].RelationType
			}
			if err := tx.Clauses(scopeUpsertClause()).Create(&edges[index]).Error; err != nil {
				return fmt.Errorf("upsert curriculum edge %q: %w", edges[index].ID, err)
			}
		}
		return nil
	})
}

func (r *mathMasteryRepository) ListCurriculum(ctx context.Context, tenantID uint64, kbID string) ([]types.MathCurriculumNode, []types.MathCurriculumEdge, error) {
	var nodes []types.MathCurriculumNode
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, kbID).
		Order("grade ASC, term ASC, sort_order ASC, id ASC").
		Find(&nodes).Error; err != nil {
		return nil, nil, err
	}
	var edges []types.MathCurriculumEdge
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, kbID).
		Order("from_node_id ASC, to_node_id ASC, id ASC").
		Find(&edges).Error; err != nil {
		return nil, nil, err
	}
	return nodes, edges, nil
}

func (r *mathMasteryRepository) UpsertSource(ctx context.Context, tenantID uint64, kbID string, source *types.MathSourceBinding) error {
	source.TenantID = tenantID
	source.KnowledgeBaseID = kbID
	if len(source.Metadata) == 0 {
		source.Metadata = types.JSON(`{}`)
	}
	return r.db.WithContext(ctx).Clauses(scopeUpsertClause()).Create(source).Error
}

func (r *mathMasteryRepository) ListSources(ctx context.Context, tenantID uint64, kbID string) ([]types.MathSourceBinding, error) {
	var sources []types.MathSourceBinding
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, kbID).
		Order("grade ASC, term ASC, source_type ASC, target_id ASC").
		Find(&sources).Error
	return sources, err
}

func (r *mathMasteryRepository) CreateAttempt(ctx context.Context, tenantID uint64, kbID string, attempt *types.MathDiagnosticAttempt) error {
	attempt.TenantID = tenantID
	attempt.KnowledgeBaseID = kbID
	if len(attempt.Scope) == 0 {
		attempt.Scope = types.JSON(`{}`)
	}
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *mathMasteryRepository) AddResponse(ctx context.Context, tenantID uint64, kbID string, response *types.MathDiagnosticResponse) error {
	response.TenantID = tenantID
	response.KnowledgeBaseID = kbID
	return r.db.WithContext(ctx).Create(response).Error
}

func (r *mathMasteryRepository) ListEvidence(ctx context.Context, tenantID uint64, kbID, nodeID string) ([]types.MathMasteryEvidence, error) {
	var evidence []types.MathMasteryEvidence
	err := r.db.WithContext(ctx).
		Model(&types.MathDiagnosticResponse{}).
		Select("question_id, question_type, attempt_id AS session_id, correct, weight").
		Where("tenant_id = ? AND knowledge_base_id = ? AND node_id = ?", tenantID, kbID, nodeID).
		Order("created_at ASC, id ASC").
		Scan(&evidence).Error
	return evidence, err
}

func scopeUpsertClause() clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "knowledge_base_id"}, {Name: "id"}},
		UpdateAll: true,
	}
}
