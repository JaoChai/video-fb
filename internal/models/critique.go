package models

import (
	"encoding/json"
	"time"
)

type ClipCritique struct {
	ClipID    string          `json:"clip_id"`
	Score     json.RawMessage `json:"score"`
	Changes   json.RawMessage `json:"changes"`
	Applied   bool            `json:"applied"`
	CreatedAt time.Time       `json:"created_at"`
}

type SkillRevision struct {
	AgentName      string    `json:"agent_name"`
	Rationale      string    `json:"rationale"`
	CritiqueWindow int       `json:"critique_window"`
	CreatedAt      time.Time `json:"created_at"`
}
