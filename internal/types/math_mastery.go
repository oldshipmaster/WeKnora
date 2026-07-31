package types

import "time"

type MathMasteryState string

const (
	MathMasteryUntested   MathMasteryState = "untested"
	MathMasteryWeak       MathMasteryState = "weak"
	MathMasteryDeveloping MathMasteryState = "developing"
	MathMasteryMastered   MathMasteryState = "mastered"
)

type MathCurriculumNode struct {
	ID                    string    `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID              uint64    `json:"tenant_id" gorm:"index;not null"`
	KnowledgeBaseID       string    `json:"knowledge_base_id" gorm:"type:varchar(36);index;not null"`
	ParentID              string    `json:"parent_id,omitempty" gorm:"type:varchar(128);index"`
	NodeType              string    `json:"node_type" gorm:"type:varchar(24);not null"`
	Grade                 int       `json:"grade" gorm:"index;not null"`
	Term                  int       `json:"term" gorm:"not null"`
	Domain                string    `json:"domain" gorm:"type:varchar(32);index;not null"`
	Title                 string    `json:"title" gorm:"type:varchar(255);not null"`
	Summary               string    `json:"summary,omitempty" gorm:"type:text"`
	SortOrder             int       `json:"sort_order" gorm:"not null;default:0"`
	ExpectedQuestionCount int       `json:"expected_question_count" gorm:"not null;default:3"`
	Metadata              JSON      `json:"metadata,omitempty" gorm:"type:json"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type MathCurriculumEdge struct {
	ID              string    `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID        uint64    `json:"tenant_id" gorm:"index;not null"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);index;not null"`
	FromNodeID      string    `json:"from_node_id" gorm:"type:varchar(128);index;not null"`
	ToNodeID        string    `json:"to_node_id" gorm:"type:varchar(128);index;not null"`
	RelationType    string    `json:"relation_type" gorm:"type:varchar(32);not null"`
	Strength        float64   `json:"strength" gorm:"not null;default:1"`
	Rationale       string    `json:"rationale,omitempty" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MathMasteryEvidence struct {
	QuestionID   string  `json:"question_id"`
	QuestionType string  `json:"question_type"`
	SessionID    string  `json:"session_id"`
	Correct      bool    `json:"correct"`
	Weight       float64 `json:"weight,omitempty"`
}

type MathMasteryAssessment struct {
	State         MathMasteryState `json:"state"`
	Score         float64          `json:"score"`
	Confidence    float64          `json:"confidence"`
	EvidenceCount int              `json:"evidence_count"`
	Reasons       []string         `json:"reasons"`
}
