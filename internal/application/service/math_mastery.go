package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/mathmastery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

type mathMasteryService struct {
	repo interfaces.MathMasteryRepository
}

func NewMathMasteryService(repo interfaces.MathMasteryRepository) interfaces.MathMasteryService {
	return &mathMasteryService{repo: repo}
}

func (s *mathMasteryService) SeedCurriculum(ctx context.Context, kbID string, nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) error {
	if strings.TrimSpace(kbID) == "" {
		return errors.New("knowledge base ID is required")
	}
	if len(nodes) == 0 {
		return errors.New("curriculum nodes are required")
	}
	if err := mathmastery.ValidateCurriculum(nodes, edges); err != nil {
		return fmt.Errorf("validate curriculum: %w", err)
	}
	return s.repo.UpsertCurriculum(ctx, types.MustTenantIDFromContext(ctx), kbID, nodes, edges)
}

func (s *mathMasteryService) GetTree(ctx context.Context, kbID string) (*types.MathMasteryTree, error) {
	if strings.TrimSpace(kbID) == "" {
		return nil, errors.New("knowledge base ID is required")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	nodes, edges, err := s.repo.ListCurriculum(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	if len(nodes) > 0 {
		if err := mathmastery.ValidateCurriculum(nodes, edges); err != nil {
			return nil, fmt.Errorf("stored curriculum is invalid: %w", err)
		}
	}
	sources, err := s.repo.ListSources(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	questionCounts, diagnosticQuestions, err := s.repo.ListQuestionCounts(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}

	views := make([]types.MathMasteryNodeView, 0, len(nodes))
	assessmentByNode := make(map[string]types.MathMasteryAssessment, len(nodes))
	for _, node := range nodes {
		evidence, listErr := s.repo.ListEvidence(ctx, tenantID, kbID, node.ID)
		if listErr != nil {
			return nil, listErr
		}
		assessment := mathmastery.AssessMastery(evidence)
		assessmentByNode[node.ID] = assessment
		views = append(views, types.MathMasteryNodeView{
			MathCurriculumNode: node,
			Assessment:         assessment,
			BlockedBy:          []string{},
			QuestionCount:      questionCounts[node.ID],
		})
	}

	viewIndex := make(map[string]int, len(views))
	for index := range views {
		viewIndex[views[index].ID] = index
	}
	for _, edge := range edges {
		if edge.RelationType != "" && edge.RelationType != "prerequisite" {
			continue
		}
		assessment := assessmentByNode[edge.FromNodeID]
		if assessment.State != types.MathMasteryMastered {
			index := viewIndex[edge.ToNodeID]
			views[index].BlockedBy = append(views[index].BlockedBy, edge.FromNodeID)
		}
	}

	tree := &types.MathMasteryTree{Nodes: views, Edges: edges, Sources: sources}
	tree.Overview = summarizeMathMastery(views, sources, diagnosticQuestions)
	return tree, nil
}

func (s *mathMasteryService) UpsertSources(ctx context.Context, kbID string, sources []types.MathSourceBinding) error {
	if strings.TrimSpace(kbID) == "" {
		return errors.New("knowledge base ID is required")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	for index := range sources {
		if sources[index].ID == "" || sources[index].TargetID == "" {
			return errors.New("source ID and target ID are required")
		}
		if err := s.repo.UpsertSource(ctx, tenantID, kbID, &sources[index]); err != nil {
			return err
		}
	}
	return nil
}

func (s *mathMasteryService) UpsertQuestions(ctx context.Context, kbID string, questions []types.MathQuestion, links []types.MathQuestionNode) error {
	if strings.TrimSpace(kbID) == "" || len(questions) == 0 {
		return errors.New("knowledge base ID and diagnostic questions are required")
	}
	questionIDs := make(map[string]struct{}, len(questions))
	for index := range questions {
		question := &questions[index]
		if strings.TrimSpace(question.ID) == "" || strings.TrimSpace(question.SourceBindingID) == "" ||
			strings.TrimSpace(question.QuestionLocator) == "" || strings.TrimSpace(question.QuestionType) == "" {
			return errors.New("question ID, source, locator and type are required")
		}
		if question.Difficulty < 0 || question.Difficulty > 1 || question.ExtractionConfidence < 0 || question.ExtractionConfidence > 1 {
			return errors.New("question difficulty and extraction confidence must be between 0 and 1")
		}
		questionIDs[question.ID] = struct{}{}
	}
	for _, link := range links {
		if _, ok := questionIDs[link.QuestionID]; !ok || strings.TrimSpace(link.NodeID) == "" {
			return errors.New("every question link must reference an imported question and curriculum node")
		}
		if link.Confidence < 0 || link.Confidence > 1 {
			return errors.New("question link confidence must be between 0 and 1")
		}
	}
	return s.repo.UpsertQuestions(ctx, types.MustTenantIDFromContext(ctx), kbID, questions, links)
}

func (s *mathMasteryService) ListQuestions(ctx context.Context, kbID, nodeID string, limit int) ([]types.MathQuestion, error) {
	if strings.TrimSpace(kbID) == "" || strings.TrimSpace(nodeID) == "" {
		return nil, errors.New("knowledge base ID and curriculum node ID are required")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 50 {
		limit = 50
	}
	return s.repo.ListQuestions(ctx, types.MustTenantIDFromContext(ctx), kbID, nodeID, limit)
}

func (s *mathMasteryService) StartAttempt(ctx context.Context, kbID, profileID string, scope types.JSON) (*types.MathDiagnosticAttempt, error) {
	if strings.TrimSpace(kbID) == "" || strings.TrimSpace(profileID) == "" {
		return nil, errors.New("knowledge base ID and profile ID are required")
	}
	now := time.Now()
	attempt := &types.MathDiagnosticAttempt{
		ID:        uuid.NewString(),
		ProfileID: profileID,
		Status:    "active",
		Scope:     scope,
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateAttempt(ctx, types.MustTenantIDFromContext(ctx), kbID, attempt); err != nil {
		return nil, err
	}
	return attempt, nil
}

func (s *mathMasteryService) SubmitResponse(ctx context.Context, kbID string, response types.MathDiagnosticResponse) (*types.MathMasteryAssessment, error) {
	if strings.TrimSpace(kbID) == "" || response.AttemptID == "" || response.QuestionID == "" || response.NodeID == "" || response.QuestionType == "" {
		return nil, errors.New("knowledge base, attempt, question, node and question type are required")
	}
	if response.ID == "" {
		response.ID = uuid.NewString()
	}
	if response.Weight <= 0 {
		response.Weight = 1
	}
	response.CreatedAt = time.Now()
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.repo.AddResponse(ctx, tenantID, kbID, &response); err != nil {
		return nil, err
	}
	evidence, err := s.repo.ListEvidence(ctx, tenantID, kbID, response.NodeID)
	if err != nil {
		return nil, err
	}
	assessment := mathmastery.AssessMastery(evidence)
	return &assessment, nil
}

func summarizeMathMastery(nodes []types.MathMasteryNodeView, sources []types.MathSourceBinding, diagnosticQuestions int) types.MathMasteryOverview {
	overview := types.MathMasteryOverview{TotalNodes: len(nodes), DiagnosticQuestions: diagnosticQuestions}
	for _, node := range nodes {
		switch node.Assessment.State {
		case types.MathMasteryUntested:
			overview.UntestedNodes++
		case types.MathMasteryWeak:
			overview.WeakNodes++
		case types.MathMasteryDeveloping:
			overview.DevelopingNodes++
		case types.MathMasteryMastered:
			overview.MasteredNodes++
		}
		if len(node.BlockedBy) > 0 {
			overview.BlockedNodes++
		}
		if node.QuestionCount > 0 {
			overview.NodesWithQuestions++
		}
	}
	if overview.TotalNodes > 0 {
		overview.CoverageRate = float64(overview.TotalNodes-overview.UntestedNodes) / float64(overview.TotalNodes)
		overview.MasteryRate = float64(overview.MasteredNodes) / float64(overview.TotalNodes)
	}
	for _, source := range sources {
		ready := source.Status == "ready"
		if source.SourceType == "textbook" {
			overview.TextbooksTotal++
			if ready {
				overview.TextbooksReady++
			}
		} else if source.SourceType == "exam" {
			overview.ExamsTotal++
			if ready {
				overview.ExamsReady++
			}
		}
	}
	return overview
}
