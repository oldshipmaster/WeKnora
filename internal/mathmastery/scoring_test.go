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

func TestAssessMasteryKeepsRepeatedQuestionsAsEvidenceAcrossSessions(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s1", Correct: true},
		{QuestionID: "q3", QuestionType: "variant", SessionID: "s1", Correct: true},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s2", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s2", Correct: true},
		{QuestionID: "q3", QuestionType: "variant", SessionID: "s2", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.State != types.MathMasteryMastered {
		t.Fatalf("state = %q, want %q after stable repeated questions across two sessions: %+v", got.State, types.MathMasteryMastered, got)
	}
	if got.EvidenceCount != 6 {
		t.Fatalf("evidence count = %d, want 6 session-question observations", got.EvidenceCount)
	}
}

func TestAssessMasteryUsesTheMostRecentTwoSessionsForCurrentState(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: false},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s1", Correct: false},
		{QuestionID: "q3", QuestionType: "variant", SessionID: "s1", Correct: false},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s2", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s2", Correct: true},
		{QuestionID: "q3", QuestionType: "variant", SessionID: "s2", Correct: true},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s3", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s3", Correct: true},
		{QuestionID: "q3", QuestionType: "variant", SessionID: "s3", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.State != types.MathMasteryMastered || got.Score != 1 {
		t.Fatalf("assessment = %+v, want recent stable sessions to override an older weak baseline", got)
	}
	if got.EvidenceCount != 9 {
		t.Fatalf("evidence count = %d, want all historical observations retained", got.EvidenceCount)
	}
}

func TestAssessMasteryChoosesRecentSessionsBeforeDeduplicatingRetries(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s2", Correct: false},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: false},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s3", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.Score != 1 {
		t.Fatalf("score = %.2f, want the latest active sessions s1 and s3 selected before same-session retries are deduplicated", got.Score)
	}
	if got.EvidenceCount != 3 {
		t.Fatalf("evidence count = %d, want three unique session-question observations across history", got.EvidenceCount)
	}
}

func TestAssessMasteryDoesNotUseEvidenceWithoutDiagnosticSessions(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", Correct: true},
		{QuestionID: "q3", QuestionType: "variant", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.State != types.MathMasteryUntested || got.Score != 0 || got.Confidence != 0 {
		t.Fatalf("assessment = %+v, want untested when no evidence belongs to a diagnostic session", got)
	}
	if got.EvidenceCount != 3 {
		t.Fatalf("evidence count = %d, want historical observations retained for audit", got.EvidenceCount)
	}
}

func TestAssessMasteryKeepsRepeatedQuestionAcrossSessionsWithoutTreatingItAsIndependent(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s2", Correct: true},
		{QuestionID: "q2", QuestionType: "variant", SessionID: "s2", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.EvidenceCount != 3 {
		t.Fatalf("evidence count = %d, want 3 session-question observations", got.EvidenceCount)
	}
	if got.State == types.MathMasteryMastered {
		t.Fatalf("state = mastered with only two independent questions")
	}
}

func TestAssessMasteryDeduplicatesDuplicateSubmissionWithinSession(t *testing.T) {
	evidence := []types.MathMasteryEvidence{
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: true},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s1", Correct: false},
		{QuestionID: "q1", QuestionType: "basic", SessionID: "s2", Correct: true},
	}

	got := AssessMastery(evidence)
	if got.EvidenceCount != 2 {
		t.Fatalf("evidence count = %d, want one observation per session-question pair", got.EvidenceCount)
	}
	if got.Score != 1 {
		t.Fatalf("score = %.2f, want the duplicate retry within s1 ignored", got.Score)
	}
	if got.Confidence >= 0.5 {
		t.Fatalf("confidence = %.2f, want one independent question to remain low-confidence", got.Confidence)
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
