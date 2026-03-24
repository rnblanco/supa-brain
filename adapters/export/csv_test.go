package export

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"supa-brain/core"
)

func TestCSVExporter_Headers(t *testing.T) {
	var buf bytes.Buffer
	exporter := &CSVExporter{}
	err := exporter.Export(context.Background(), &buf, []core.Memory{})
	require.NoError(t, err)

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1) // only header row
	assert.Equal(t, []string{"id", "title", "type", "project", "scope", "topic_key", "content", "created_at"}, records[0])
}

func TestCSVExporter_WithData(t *testing.T) {
	tk := "arch/auth"
	memories := []core.Memory{
		{
			ID:        42,
			Title:     "Auth decision",
			Content:   "**What**: JWT",
			Type:      "decision",
			Project:   "vettrack-pro",
			Scope:     "project",
			TopicKey:  &tk,
			CreatedAt: time.Date(2026, 3, 20, 7, 8, 10, 0, time.UTC),
		},
	}

	var buf bytes.Buffer
	exporter := &CSVExporter{}
	err := exporter.Export(context.Background(), &buf, memories)
	require.NoError(t, err)

	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2) // header + 1 data row

	row := records[1]
	assert.Equal(t, "42", row[0])
	assert.Equal(t, "Auth decision", row[1])
	assert.Equal(t, "arch/auth", row[5])
}
