package mathmastery

import (
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestBuildCurriculumSeedConvertsPrerequisiteEdges(t *testing.T) {
	raw := []byte(`{
		"nodes":[{"id":"count","node_type":"concept","grade":1,"term":1,"domain":"number","title":"数一数"}],
		"edges":[{"from":"count","to":"add","relation_type":"prerequisite","strength":0.8}]
	}`)

	seed, err := BuildCurriculumSeed(raw)
	require.NoError(t, err)
	require.Len(t, seed.Nodes, 1)
	require.Equal(t, "count", seed.Nodes[0].ID)
	require.Equal(t, []types.MathCurriculumEdge{{
		FromNodeID:   "count",
		ToNodeID:     "add",
		RelationType: "prerequisite",
		Strength:     0.8,
	}}, seed.Edges)
}

func TestBuildSourceBindingsPreservesExactMissingExamTargets(t *testing.T) {
	manifest := Manifest{Entries: []ManifestEntry{
		{
			TargetID: "rj-g1-s1-textbook", MaterialID: "material-1", Kind: MaterialTextbook,
			Status: StatusFound, Title: "人教版小学数学一年级上册", Edition: "人教版",
			Grade: 1, Term: 1, Path: "/books/g1.pdf", ContentHash: "abc", Size: 42,
		},
		{
			TargetID: "rj-g1-s2-xueba-2026-spring", Kind: MaterialExam,
			Status: StatusMissing, Title: "2026春人教版一年级下册《学霸提优大试卷》数学",
			Edition: "人教版", Grade: 1, Term: 2, SchoolYear: 2026, Season: "spring",
		},
	}}

	bindings := BuildSourceBindings(manifest, map[string]UploadedKnowledge{
		"rj-g1-s1-textbook": {ID: "knowledge-1", Status: StatusProcessing},
	})
	require.Len(t, bindings, 2)
	require.Equal(t, "material-1", bindings[0].ID)
	require.Equal(t, "knowledge-1", bindings[0].KnowledgeID)
	require.Equal(t, "processing", bindings[0].Status)
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(bindings[0].Metadata, &metadata))
	require.Equal(t, float64(42), metadata["size"])
	require.Equal(t, "rj-g1-s2-xueba-2026-spring", bindings[1].ID)
	require.Equal(t, "missing", bindings[1].Status)
	require.Equal(t, 2026, bindings[1].SchoolYear)
	require.Contains(t, bindings[1].ErrorMessage, "未找到严格匹配")

	_, err := json.Marshal(bindings)
	require.NoError(t, err)
}
