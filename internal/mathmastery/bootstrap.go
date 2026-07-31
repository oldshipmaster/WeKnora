package mathmastery

import (
	"encoding/json"
	"fmt"

	"github.com/Tencent/WeKnora/internal/types"
)

// CurriculumSeed is the request payload accepted by the math mastery seed API.
type CurriculumSeed struct {
	Nodes []types.MathCurriculumNode `json:"nodes"`
	Edges []types.MathCurriculumEdge `json:"edges"`
}

// UploadedKnowledge records the knowledge object produced by one material upload.
type UploadedKnowledge struct {
	ID     string
	Status MaterialStatus
}

// BuildCurriculumSeed adapts the checked-in curriculum data format to the API
// format without weakening graph validation in the service layer.
func BuildCurriculumSeed(data []byte) (CurriculumSeed, error) {
	var source struct {
		Nodes []types.MathCurriculumNode `json:"nodes"`
		Edges []struct {
			From         string  `json:"from"`
			To           string  `json:"to"`
			RelationType string  `json:"relation_type"`
			Strength     float64 `json:"strength"`
			Rationale    string  `json:"rationale,omitempty"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return CurriculumSeed{}, fmt.Errorf("decode curriculum: %w", err)
	}
	if len(source.Nodes) == 0 {
		return CurriculumSeed{}, fmt.Errorf("curriculum has no nodes")
	}

	seed := CurriculumSeed{Nodes: source.Nodes, Edges: make([]types.MathCurriculumEdge, 0, len(source.Edges))}
	for _, edge := range source.Edges {
		seed.Edges = append(seed.Edges, types.MathCurriculumEdge{
			FromNodeID:   edge.From,
			ToNodeID:     edge.To,
			RelationType: edge.RelationType,
			Strength:     edge.Strength,
			Rationale:    edge.Rationale,
		})
	}
	return seed, nil
}

// BuildSourceBindings produces one auditable source row for every exact target.
// A missing 2026 paper therefore remains visible instead of being silently
// replaced by an older edition.
func BuildSourceBindings(manifest Manifest, uploaded map[string]UploadedKnowledge) []types.MathSourceBinding {
	bindings := make([]types.MathSourceBinding, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		id := entry.MaterialID
		if id == "" {
			id = entry.TargetID
		}
		status := entry.Status
		knowledgeID := ""
		if knowledge, ok := uploaded[entry.TargetID]; ok {
			knowledgeID = knowledge.ID
			status = knowledge.Status
			if status == "" {
				status = StatusUploaded
			}
		}
		metadata, _ := json.Marshal(map[string]any{
			"size":        entry.Size,
			"modified_at": entry.ModifiedAt,
		})
		binding := types.MathSourceBinding{
			ID:          id,
			TargetID:    entry.TargetID,
			KnowledgeID: knowledgeID,
			SourceType:  string(entry.Kind),
			Status:      string(status),
			Title:       entry.Title,
			Edition:     entry.Edition,
			SchoolYear:  entry.SchoolYear,
			Season:      entry.Season,
			Grade:       entry.Grade,
			Term:        entry.Term,
			ContentHash: entry.ContentHash,
			SourcePath:  entry.Path,
			Metadata:    types.JSON(metadata),
		}
		if entry.Status == StatusMissing {
			binding.ErrorMessage = "未找到严格匹配目标版本的文件；未使用旧版材料替代"
		}
		bindings = append(bindings, binding)
	}
	return bindings
}
