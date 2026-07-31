package repository

import (
	"context"
	"errors"
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing types.MathSourceBinding
		err := tx.Where("tenant_id = ? AND knowledge_base_id = ? AND target_id = ?", tenantID, kbID, source.TargetID).
			Take(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find source target %q: %w", source.TargetID, err)
		}
		if err == nil && existing.ID != source.ID {
			if updateErr := tx.Model(&types.MathQuestion{}).
				Where("tenant_id = ? AND knowledge_base_id = ? AND source_binding_id = ?", tenantID, kbID, existing.ID).
				Update("source_binding_id", source.ID).Error; updateErr != nil {
				return fmt.Errorf("migrate question source %q to %q: %w", existing.ID, source.ID, updateErr)
			}
			if updateErr := tx.Model(&types.MathSourceBinding{}).
				Where("tenant_id = ? AND knowledge_base_id = ? AND id = ?", tenantID, kbID, existing.ID).
				Update("id", source.ID).Error; updateErr != nil {
				return fmt.Errorf("migrate source ID %q to %q: %w", existing.ID, source.ID, updateErr)
			}
		}
		if err := tx.Clauses(scopeUpsertClause()).Create(source).Error; err != nil {
			return fmt.Errorf("upsert source %q: %w", source.ID, err)
		}
		return nil
	})
}

func (r *mathMasteryRepository) ListSources(ctx context.Context, tenantID uint64, kbID string) ([]types.MathSourceBinding, error) {
	var sources []types.MathSourceBinding
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, kbID).
		Order("grade ASC, term ASC, source_type ASC, target_id ASC").
		Find(&sources).Error
	return sources, err
}

func (r *mathMasteryRepository) UpsertQuestions(ctx context.Context, tenantID uint64, kbID string, questions []types.MathQuestion, links []types.MathQuestionNode) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index := range questions {
			questions[index].TenantID = tenantID
			questions[index].KnowledgeBaseID = kbID
			if len(questions[index].ScoringRule) == 0 {
				questions[index].ScoringRule = types.JSON(`{}`)
			}
			if err := tx.Clauses(scopeUpsertClause()).Create(&questions[index]).Error; err != nil {
				return fmt.Errorf("upsert math question %q: %w", questions[index].ID, err)
			}
		}
		for index := range links {
			links[index].TenantID = tenantID
			links[index].KnowledgeBaseID = kbID
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "knowledge_base_id"}, {Name: "question_id"}, {Name: "node_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"is_primary", "confidence"}),
			}).Create(&links[index]).Error; err != nil {
				return fmt.Errorf("upsert math question link %q -> %q: %w", links[index].QuestionID, links[index].NodeID, err)
			}
		}
		return nil
	})
}

func (r *mathMasteryRepository) ListQuestions(ctx context.Context, tenantID uint64, kbID, nodeID string, limit int) ([]types.MathQuestion, error) {
	var questions []types.MathQuestion
	err := r.db.WithContext(ctx).
		Model(&types.MathQuestion{}).
		Select("math_questions.*").
		Joins("JOIN math_question_nodes AS question_nodes ON question_nodes.tenant_id = math_questions.tenant_id AND question_nodes.knowledge_base_id = math_questions.knowledge_base_id AND question_nodes.question_id = math_questions.id").
		Where("math_questions.tenant_id = ? AND math_questions.knowledge_base_id = ? AND question_nodes.node_id = ?", tenantID, kbID, nodeID).
		Order("math_questions.difficulty ASC, math_questions.question_locator ASC, math_questions.id ASC").
		Limit(limit).
		Find(&questions).Error
	return questions, err
}

func (r *mathMasteryRepository) ListQuestionCounts(ctx context.Context, tenantID uint64, kbID string) (map[string]int, int, error) {
	type nodeQuestionCount struct {
		NodeID string
		Count  int
	}
	var rows []nodeQuestionCount
	validQuestions := func() *gorm.DB {
		return r.db.WithContext(ctx).
			Table("math_question_nodes AS question_nodes").
			Joins("JOIN math_questions ON math_questions.tenant_id = question_nodes.tenant_id AND math_questions.knowledge_base_id = question_nodes.knowledge_base_id AND math_questions.id = question_nodes.question_id").
			Where("question_nodes.tenant_id = ? AND question_nodes.knowledge_base_id = ?", tenantID, kbID)
	}
	err := validQuestions().
		Select("question_nodes.node_id, COUNT(DISTINCT question_nodes.question_id) AS count").
		Group("question_nodes.node_id").
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.NodeID] = row.Count
	}
	var total int64
	if err := validQuestions().Distinct("question_nodes.question_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	return counts, int(total), nil
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
