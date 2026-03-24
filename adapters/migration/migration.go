package migration

import (
	"encoding/json"
	"fmt"
	"time"

	"memory-server/core"
)

type engramExport struct {
	Observations []engramObs `json:"observations"`
}

type engramObs struct {
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Type      string  `json:"type"`
	Project   string  `json:"project"`
	Scope     string  `json:"scope"`
	TopicKey  *string `json:"topic_key"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ParseEngram parses an Engram export JSON and returns []core.Memory.
// Embeddings are NOT set — caller must generate them before saving.
func ParseEngram(data []byte) ([]core.Memory, error) {
	var export engramExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parse engram export: %w", err)
	}

	memories := make([]core.Memory, 0, len(export.Observations))
	for _, obs := range export.Observations {
		createdAt, _ := time.Parse(time.RFC3339, obs.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, obs.UpdatedAt)

		memories = append(memories, core.Memory{
			Title:     obs.Title,
			Content:   obs.Content,
			Type:      obs.Type,
			Project:   obs.Project,
			Scope:     obs.Scope,
			TopicKey:  obs.TopicKey,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}
	return memories, nil
}
