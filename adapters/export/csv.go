package export

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"memory-server/core"
)

// CSVExporter writes memories as CSV.
type CSVExporter struct{}

func (e *CSVExporter) Export(_ context.Context, w io.Writer, memories []core.Memory) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	cw.Write([]string{"id", "title", "type", "project", "scope", "topic_key", "content", "created_at"})

	for _, m := range memories {
		topicKey := ""
		if m.TopicKey != nil {
			topicKey = *m.TopicKey
		}
		cw.Write([]string{
			fmt.Sprintf("%d", m.ID),
			m.Title,
			m.Type,
			m.Project,
			m.Scope,
			topicKey,
			m.Content,
			m.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return cw.Error()
}
