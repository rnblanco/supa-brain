package export

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"memory-server/core"
)

// JSONExporter writes memories in Engram-compatible JSON format.
type JSONExporter struct{}

type jsonExportFile struct {
	Version      string        `json:"version"`
	ExportedAt   string        `json:"exported_at"`
	Observations []jsonObsItem `json:"observations"`
}

type jsonObsItem struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Type      string  `json:"type"`
	Project   string  `json:"project"`
	Scope     string  `json:"scope"`
	TopicKey  *string `json:"topic_key"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func (e *JSONExporter) Export(_ context.Context, w io.Writer, memories []core.Memory) error {
	obs := make([]jsonObsItem, len(memories))
	for i, m := range memories {
		obs[i] = jsonObsItem{
			ID:        m.ID,
			Title:     m.Title,
			Content:   m.Content,
			Type:      m.Type,
			Project:   m.Project,
			Scope:     m.Scope,
			TopicKey:  m.TopicKey,
			CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: m.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonExportFile{
		Version:      "1.0",
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		Observations: obs,
	})
}
