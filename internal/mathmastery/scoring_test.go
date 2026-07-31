package mathmastery

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestAssessMasteryWithoutEvidenceIsUntested(t *testing.T) {
	got := AssessMastery(nil)
	if got.State != types.MathMasteryUntested || got.Score != 0 || got.Confidence != 0 || got.EvidenceCount != 0 {
		t.Fatalf("AssessMastery(nil) = %+v, want empty untested assessment", got)
	}
}

func TestAssessMasteryWithIncorrectEvidenceIsWeak(t *testing.T) {
	got := AssessMastery([]types.MathMasteryEvidence{{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: false}})
	if got.State != types.MathMasteryWeak {
		t.Fatalf("state = %q, want %q", got.State, types.MathMasteryWeak)
	}
	if got.EvidenceCount != 1 {
		t.Fatalf("evidence count = %d, want 1", got.EvidenceCount)
	}
}

func TestAssessMasteryCapsSingleCorrectAnswerAtDeveloping(t *testing.T) {
	got := AssessMastery([]types.MathMasteryEvidence{{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true}})
	if got.State != types.MathMasteryDeveloping {
		t.Fatalf("state = %q, want %q", got.State, types.MathMasteryDeveloping)
	}
	if got.Confidence >= 0.5 {
		t.Fatalf("confidence = %.2f, want less than 0.5 for one answer", got.Confidence)
	}
}

func TestAssessMasteryRequiresIndependentQuestionsTypesAndSessions(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s1", Correct: true},
		{QuestionID: "q3", QuestionType: "variant", SessionID: "s2", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.State != types.MathMasteryMastered {
		t.Fatalf("state = %q, want %q: %+v", got.State, types.MathMasteryMastered, got)
	}
	if got.Score != 1 || got.Confidence != 1 || got.EvidenceCount != 3 {
		t.Fatalf("assessment = %+v, want full score/confidence with 3 evidence", got)
	}
}

func TestAssessMasteryDeduplicatesRepeatedQuestion(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s2", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s2", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.EvidenceCount != 2 {
		t.Fatalf("evidence count = %d, want 2 unique questions", got.EvidenceCount)
	}
	if got.State == types.MathMasteryMastered {
		t.Fatalf("state = mastered with only two independent questions")
	}
}

func TestAssessMasteryRequiresTwoQuestionTypesAndSessions(t *testing.T) {
	tests := []struct {
		name     string
		evidence []types.MathMasteryEvidence
	}{
		{
			name: "one type",
			evidence: []types.MathMasteryEvidence{
				{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
				{QuestionID: "q2", QuestionType: "basic", SessionID: "s1", Correct: true},
				{QuestionID: "q3", QuestionType: "basic", SessionID: "s2", Correct: true},
			},
		},
		{
			name: "one session",
			evidence: []types.MathMasteryEvidence{
				{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
				{QuestionID: "q2", QuestionType: "variant", SessionID: "s1", Correct: true},
				{QuestionID: "q3", QuestionType: "transfer", SessionID: "s1", Correct: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AssessMastery(tt.evidence); got.State == types.MathMasteryMastered {
				t.Fatalf("state = mastered without sufficient diversity: %+v", got)
			}
		})
	}
}
