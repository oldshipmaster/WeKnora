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
	TenantID              uint64    `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;index;not null"`
	KnowledgeBaseID       string    `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;index;not null"`
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
	TenantID        uint64    `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;index;not null"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;index;not null"`
	FromNodeID      string    `json:"from_node_id" gorm:"type:varchar(128);index;not null"`
	ToNodeID        string    `json:"to_node_id" gorm:"type:varchar(128);index;not null"`
	RelationType    string    `json:"relation_type" gorm:"type:varchar(32);not null"`
	Strength        float64   `json:"strength" gorm:"not null;default:1"`
	Rationale       string    `json:"rationale,omitempty" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MathSourceBinding struct {
	ID              string    `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID        uint64    `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;index;not null"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;index;not null"`
	TargetID        string    `json:"target_id" gorm:"type:varchar(128);index;not null"`
	NodeID          string    `json:"node_id,omitempty" gorm:"type:varchar(128);index"`
	KnowledgeID     string    `json:"knowledge_id,omitempty" gorm:"type:varchar(36);index"`
	ChunkID         string    `json:"chunk_id,omitempty" gorm:"type:varchar(36);index"`
	SourceType      string    `json:"source_type" gorm:"type:varchar(24);index;not null"`
	Status          string    `json:"status" gorm:"type:varchar(24);index;not null"`
	Title           string    `json:"title,omitempty" gorm:"type:varchar(255)"`
	Edition         string    `json:"edition" gorm:"type:varchar(64)"`
	SchoolYear      int       `json:"school_year,omitempty"`
	Season          string    `json:"season,omitempty" gorm:"type:varchar(16)"`
	Grade           int       `json:"grade" gorm:"index"`
	Term            int       `json:"term"`
	PageFrom        int       `json:"page_from,omitempty"`
	PageTo          int       `json:"page_to,omitempty"`
	ContentHash     string    `json:"content_hash,omitempty" gorm:"type:varchar(64);index"`
	SourcePath      string    `json:"source_path,omitempty" gorm:"type:text"`
	ErrorMessage    string    `json:"error_message,omitempty" gorm:"type:text"`
	Metadata        JSON      `json:"metadata,omitempty" gorm:"type:json"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MathQuestion struct {
	ID                   string    `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID             uint64    `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;index;not null"`
	KnowledgeBaseID      string    `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;index;not null"`
	SourceBindingID      string    `json:"source_binding_id,omitempty" gorm:"type:varchar(128);index"`
	QuestionLocator      string    `json:"question_locator" gorm:"type:varchar(255);not null"`
	AnswerLocator        string    `json:"answer_locator,omitempty" gorm:"type:varchar(255)"`
	QuestionType         string    `json:"question_type" gorm:"type:varchar(32);index;not null"`
	Difficulty           float64   `json:"difficulty"`
	ScoringRule          JSON      `json:"scoring_rule,omitempty" gorm:"type:json"`
	ExtractionConfidence float64   `json:"extraction_confidence"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type MathQuestionNode struct {
	TenantID        uint64    `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;not null"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;not null"`
	QuestionID      string    `json:"question_id" gorm:"type:varchar(128);primaryKey"`
	NodeID          string    `json:"node_id" gorm:"type:varchar(128);primaryKey"`
	IsPrimary       bool      `json:"is_primary"`
	Confidence      float64   `json:"confidence"`
	CreatedAt       time.Time `json:"created_at"`
}

type MathDiagnosticAttempt struct {
	ID              string     `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID        uint64     `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;index;not null"`
	KnowledgeBaseID string     `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;index;not null"`
	ProfileID       string     `json:"profile_id" gorm:"type:varchar(128);index;not null"`
	Status          string     `json:"status" gorm:"type:varchar(24);index;not null"`
	Scope           JSON       `json:"scope,omitempty" gorm:"type:json"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type MathDiagnosticResponse struct {
	ID              string    `json:"id" gorm:"type:varchar(128);primaryKey"`
	TenantID        uint64    `json:"tenant_id" gorm:"primaryKey;autoIncrement:false;index;not null"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);primaryKey;index;not null"`
	AttemptID       string    `json:"attempt_id" gorm:"type:varchar(128);index;not null"`
	QuestionID      string    `json:"question_id" gorm:"type:varchar(128);index;not null"`
	NodeID          string    `json:"node_id" gorm:"type:varchar(128);index;not null"`
	QuestionType    string    `json:"question_type" gorm:"type:varchar(32);not null"`
	Correct         bool      `json:"correct"`
	Weight          float64   `json:"weight" gorm:"not null;default:1"`
	DurationMS      int64     `json:"duration_ms,omitempty"`
	SelfConfidence  float64   `json:"self_confidence,omitempty"`
	ErrorType       string    `json:"error_type,omitempty" gorm:"type:varchar(64)"`
	CreatedAt       time.Time `json:"created_at"`
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

type MathMasteryNodeView struct {
	MathCurriculumNode
	Assessment    MathMasteryAssessment `json:"assessment"`
	BlockedBy     []string              `json:"blocked_by"`
	QuestionCount int                   `json:"question_count"`
}

type MathMasteryOverview struct {
	TotalNodes          int     `json:"total_nodes"`
	UntestedNodes       int     `json:"untested_nodes"`
	WeakNodes           int     `json:"weak_nodes"`
	DevelopingNodes     int     `json:"developing_nodes"`
	MasteredNodes       int     `json:"mastered_nodes"`
	BlockedNodes        int     `json:"blocked_nodes"`
	CoverageRate        float64 `json:"coverage_rate"`
	MasteryRate         float64 `json:"mastery_rate"`
	TextbooksReady      int     `json:"textbooks_ready"`
	TextbooksTotal      int     `json:"textbooks_total"`
	ExamsReady          int     `json:"exams_ready"`
	ExamsTotal          int     `json:"exams_total"`
	DiagnosticQuestions int     `json:"diagnostic_questions"`
	NodesWithQuestions  int     `json:"nodes_with_questions"`
}

type MathMasteryTree struct {
	Overview MathMasteryOverview   `json:"overview"`
	Nodes    []MathMasteryNodeView `json:"nodes"`
	Edges    []MathCurriculumEdge  `json:"edges"`
	Sources  []MathSourceBinding   `json:"sources"`
}
