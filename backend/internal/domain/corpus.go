package domain

import "time"

const CorpusSchemaVersion = "1.0"

// CorpusPage is a paginated, anonymized export suitable for evaluation and
// supervised training. Player identifiers are deliberately omitted.
type CorpusPage struct {
	SchemaVersion string           `json:"schemaVersion"`
	GeneratedAt   time.Time        `json:"generatedAt"`
	Items         []CorpusDialogue `json:"items"`
	Total         int              `json:"total"`
	Limit         int              `json:"limit"`
	Offset        int              `json:"offset"`
}

type CorpusDialogue struct {
	SchemaVersion    string        `json:"schemaVersion"`
	Origin           string        `json:"origin"`
	DialogueID       string        `json:"dialogueId"`
	ParentDialogueID string        `json:"parentDialogueId,omitempty"`
	ForkedFromTurn   *int          `json:"forkedFromTurn,omitempty"`
	Status           string        `json:"status"`
	StartedAt        time.Time     `json:"startedAt"`
	FinishedAt       *time.Time    `json:"finishedAt,omitempty"`
	Scenario         Scenario      `json:"scenario"`
	InitialMessage   CorpusMessage `json:"initialMessage"`
	Turns            []CorpusTurn  `json:"turns"`
	Result           *Result       `json:"result,omitempty"`
	Quality          CorpusQuality `json:"quality"`
}

type CorpusQuality struct {
	Complete bool     `json:"complete"`
	Issues   []string `json:"issues"`
}

type CorpusMessage struct {
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type CorpusTurn struct {
	Turn     int             `json:"turn"`
	Player   CorpusMessage   `json:"player"`
	Opponent CorpusMessage   `json:"opponent"`
	Analysis TurnAnalysis    `json:"analysis"`
	Before   *CorpusSnapshot `json:"before,omitempty"`
	After    *CorpusSnapshot `json:"after,omitempty"`
}

type CorpusSnapshot struct {
	TrustScore    int          `json:"trustScore"`
	ArgumentScore int          `json:"argumentScore"`
	PressureScore int          `json:"pressureScore"`
	State         SessionState `json:"state"`
}
