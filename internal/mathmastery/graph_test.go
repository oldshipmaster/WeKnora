package mathmastery

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestValidateCurriculumRejectsEdgeWithMissingNode(t *testing.T) {
	nodes := []types.MathCurriculumNode{{ID: "number-5", Grade: 1, Term: 1}}
	edges := []types.MathCurriculumEdge{{FromNodeID: "missing", ToNodeID: "number-5", RelationType: "prerequisite"}}

	err := ValidateCurriculum(nodes, edges)
	if !errors.Is(err, ErrUnknownCurriculumNode) {
		t.Fatalf("ValidateCurriculum() error = %v, want ErrUnknownCurriculumNode", err)
	}
}

func TestValidateCurriculumRejectsCycle(t *testing.T) {
	nodes := []types.MathCurriculumNode{
		{ID: "a", Grade: 1, Term: 1},
		{ID: "b", Grade: 1, Term: 1},
		{ID: "c", Grade: 1, Term: 1},
	}
	edges := []types.MathCurriculumEdge{
		{FromNodeID: "a", ToNodeID: "b", RelationType: "prerequisite"},
		{FromNodeID: "b", ToNodeID: "c", RelationType: "prerequisite"},
		{FromNodeID: "c", ToNodeID: "a", RelationType: "prerequisite"},
	}

	err := ValidateCurriculum(nodes, edges)
	if !errors.Is(err, ErrCurriculumCycle) {
		t.Fatalf("ValidateCurriculum() error = %v, want ErrCurriculumCycle", err)
	}
}

func TestValidateCurriculumRejectsBackwardGradePrerequisite(t *testing.T) {
	nodes := []types.MathCurriculumNode{
		{ID: "grade-six", Grade: 6, Term: 1},
		{ID: "grade-five", Grade: 5, Term: 2},
	}
	edges := []types.MathCurriculumEdge{{FromNodeID: "grade-six", ToNodeID: "grade-five", RelationType: "prerequisite"}}

	err := ValidateCurriculum(nodes, edges)
	if !errors.Is(err, ErrBackwardPrerequisite) {
		t.Fatalf("ValidateCurriculum() error = %v, want ErrBackwardPrerequisite", err)
	}
}

func TestTopologicalOrderAllowsCrossGradePrerequisites(t *testing.T) {
	nodes := []types.MathCurriculumNode{
		{ID: "fraction-multiply", Grade: 6, Term: 1, SortOrder: 2},
		{ID: "fraction-meaning", Grade: 5, Term: 2, SortOrder: 1},
		{ID: "fraction-initial", Grade: 3, Term: 1, SortOrder: 1},
	}
	edges := []types.MathCurriculumEdge{
		{FromNodeID: "fraction-initial", ToNodeID: "fraction-meaning", RelationType: "prerequisite"},
		{FromNodeID: "fraction-meaning", ToNodeID: "fraction-multiply", RelationType: "prerequisite"},
	}

	got, err := TopologicalOrder(nodes, edges)
	if err != nil {
		t.Fatalf("TopologicalOrder() error = %v", err)
	}
	want := []string{"fraction-initial", "fraction-meaning", "fraction-multiply"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TopologicalOrder() = %v, want %v", got, want)
	}
}
