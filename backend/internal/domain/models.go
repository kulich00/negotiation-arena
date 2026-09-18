package domain

import "time"

type Scenario struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Sphere         string `json:"sphere"`
	Topic          string `json:"topic"`
	Difficulty     string `json:"difficulty"`
	OpponentRole   string `json:"opponentRole"`
	OpponentTone   string `json:"opponentTone"`
	PlayerGoal     string `json:"playerGoal"`
	OpponentGoal   string `json:"opponentGoal"`
	InitialMessage string `json:"initialMessage"`
}

type Session struct {
	ID             string    `json:"id"`
	ScenarioID     string    `json:"scenarioId"`
	Status         string    `json:"status"`
	Turn           int       `json:"turn"`
	TrustScore     int       `json:"trustScore"`
	ArgumentScore  int       `json:"argumentScore"`
	PressureScore  int       `json:"pressureScore"`
	InitialMessage string    `json:"initialMessage,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
}

type Result struct {
	SessionID       string   `json:"sessionId"`
	FinalScore      int      `json:"finalScore"`
	Outcome         string   `json:"outcome"`
	Strengths       []string `json:"strengths"`
	Mistakes        []string `json:"mistakes"`
	Recommendations []string `json:"recommendations"`
}
