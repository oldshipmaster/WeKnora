package mathmastery

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Tencent/WeKnora/internal/types"
)

var (
	ErrUnknownCurriculumNode = errors.New("curriculum edge references an unknown node")
	ErrCurriculumCycle       = errors.New("curriculum contains a prerequisite cycle")
	ErrBackwardPrerequisite  = errors.New("curriculum prerequisite points backward across grade or term")
)

func ValidateCurriculum(nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) error {
	_, err := TopologicalOrder(nodes, edges)
	return err
}

func TopologicalOrder(nodes []types.MathCurriculumNode, edges []types.MathCurriculumEdge) ([]string, error) {
	byID := make(map[string]types.MathCurriculumNode, len(nodes))
	indegree := make(map[string]int, len(nodes))
	adjacency := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
		indegree[node.ID] = 0
	}

	for _, edge := range edges {
		if edge.RelationType != "" && edge.RelationType != "prerequisite" {
			continue
		}
		from, fromOK := byID[edge.FromNodeID]
		to, toOK := byID[edge.ToNodeID]
		if !fromOK || !toOK {
			return nil, fmt.Errorf("%w: %s -> %s", ErrUnknownCurriculumNode, edge.FromNodeID, edge.ToNodeID)
		}
		if from.Grade > to.Grade || (from.Grade == to.Grade && from.Term > to.Term) {
			return nil, fmt.Errorf("%w: %s -> %s", ErrBackwardPrerequisite, edge.FromNodeID, edge.ToNodeID)
		}
		adjacency[edge.FromNodeID] = append(adjacency[edge.FromNodeID], edge.ToNodeID)
		indegree[edge.ToNodeID]++
	}

	ready := make([]string, 0, len(nodes))
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sortNodeIDs(ready, byID)

	order := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, next := range adjacency[id] {
			indegree[next]--
			if indegree[next] == 0 {
				ready = append(ready, next)
			}
		}
		sortNodeIDs(ready, byID)
	}

	if len(order) != len(nodes) {
		return nil, ErrCurriculumCycle
	}
	return order, nil
}

func sortNodeIDs(ids []string, nodes map[string]types.MathCurriculumNode) {
	sort.Slice(ids, func(i, j int) bool {
		left, right := nodes[ids[i]], nodes[ids[j]]
		if left.Grade != right.Grade {
			return left.Grade < right.Grade
		}
		if left.Term != right.Term {
			return left.Term < right.Term
		}
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return left.ID < right.ID
	})
}
