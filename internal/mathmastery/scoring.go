package mathmastery

import (
	"math"
	"sort"
	"strconv"

	"github.com/Tencent/WeKnora/internal/types"
)

func AssessMastery(evidence []types.MathMasteryEvidence) types.MathMasteryAssessment {
	unique := deduplicateEvidence(evidence)
	if len(unique) == 0 {
		return types.MathMasteryAssessment{
			State:   types.MathMasteryUntested,
			Reasons: []string{"还没有可用于判断的诊断作答"},
		}
	}

	typesSeen := make(map[string]struct{})
	sessions := make(map[string][]types.MathMasteryEvidence)
	var earned, possible float64
	for _, item := range unique {
		weight := item.Weight
		if weight <= 0 {
			weight = 1
		}
		possible += weight
		if item.Correct {
			earned += weight
		}
		if item.QuestionType != "" {
			typesSeen[item.QuestionType] = struct{}{}
		}
		if item.SessionID != "" {
			sessions[item.SessionID] = append(sessions[item.SessionID], item)
		}
	}

	score := earned / possible
	confidence := math.Min(float64(len(unique))/3, 1)
	confidence = math.Min(confidence, math.Min(float64(len(typesSeen))/2, 1))
	confidence = math.Min(confidence, math.Min(float64(len(sessions))/2, 1))

	assessment := types.MathMasteryAssessment{
		State:         types.MathMasteryDeveloping,
		Score:         score,
		Confidence:    confidence,
		EvidenceCount: len(unique),
	}

	if score < 0.6 {
		assessment.State = types.MathMasteryWeak
		assessment.Reasons = []string{"当前正确率低于稳固掌握所需水平"}
		return assessment
	}

	if len(unique) >= 3 && len(typesSeen) >= 2 && len(sessions) >= 2 && score >= 0.8 && stableAcrossSessions(sessions) {
		assessment.State = types.MathMasteryMastered
		assessment.Confidence = 1
		assessment.Reasons = []string{"至少三道独立题、两种题型且跨两次诊断表现稳定"}
		return assessment
	}

	assessment.Reasons = developingReasons(len(unique), len(typesSeen), len(sessions))
	return assessment
}

func deduplicateEvidence(evidence []types.MathMasteryEvidence) []types.MathMasteryEvidence {
	byQuestion := make(map[string]types.MathMasteryEvidence, len(evidence))
	order := make([]string, 0, len(evidence))
	for index, item := range evidence {
		key := item.QuestionID
		if key == "" {
			key = "anonymous-" + strconv.Itoa(index)
		}
		if _, exists := byQuestion[key]; exists {
			continue
		}
		byQuestion[key] = item
		order = append(order, key)
	}
	result := make([]types.MathMasteryEvidence, 0, len(order))
	for _, key := range order {
		result = append(result, byQuestion[key])
	}
	return result
}

func stableAcrossSessions(sessions map[string][]types.MathMasteryEvidence) bool {
	for _, items := range sessions {
		correct := 0
		for _, item := range items {
			if item.Correct {
				correct++
			}
		}
		if float64(correct)/float64(len(items)) < 0.67 {
			return false
		}
	}
	return true
}

func developingReasons(questionCount, typeCount, sessionCount int) []string {
	reasons := make([]string, 0, 3)
	if questionCount < 3 {
		reasons = append(reasons, "还需要至少三道独立题作为证据")
	}
	if typeCount < 2 {
		reasons = append(reasons, "还需要覆盖至少两种题型")
	}
	if sessionCount < 2 {
		reasons = append(reasons, "还需要在第二次诊断中验证稳定性")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "已有证据尚未达到稳定掌握阈值")
	}
	sort.Strings(reasons)
	return reasons
}
