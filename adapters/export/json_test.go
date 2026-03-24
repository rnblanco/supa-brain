package export

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"supa-brain/core"
)

func TestJSONExporter_EngramCompatible(t *testing.T) {
	tk := "architecture/auth"
	memories := []core.Memory{
		{
			ID:        1,
			Title:     "Auth decision",
			Content:   "**What**: Used JWT",
			Type:      "decision",
			Project:   "vettrack-pro",
			Scope:     "project",
			TopicKey:  &tk,
			CreatedAt: time.Date(2026, 3, 20, 7, 8, 10, 0, time.UTC),
			UpdatedAt: time.Date(2026, 3, 20, 7, 8, 10, 0, time.UTC),
		},
		{
			ID:        2,
			Title:     "No topic key memory",
			Content:   "**What**: something",
			Type:      "bugfix",
			Project:   "personal",
			Scope:     "personal",
			TopicKey:  nil,
			CreatedAt: time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC),
		},
	}

	var buf bytes.Buffer
	exporter := &JSONExporter{}
	err := exporter.Export(context.Background(), &buf, memories)
	require.NoError(t, err)

	// Parse result
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "1.0", result["version"])
	assert.NotEmpty(t, result["exported_at"])

	obs := result["observations"].([]interface{})
	assert.Len(t, obs, 2)

	first := obs[0].(map[string]interface{})
	assert.Equal(t, "Auth decision", first["title"])
	assert.Equal(t, "architecture/auth", first["topic_key"])
	assert.Equal(t, "2026-03-20T07:08:10Z", first["created_at"])

	second := obs[1].(map[string]interface{})
	assert.Nil(t, second["topic_key"])
}

func TestJSONExporter_EmptyMemories(t *testing.T) {
	var buf bytes.Buffer
	exporter := &JSONExporter{}
	err := exporter.Export(context.Background(), &buf, []core.Memory{})
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	obs := result["observations"].([]interface{})
	assert.Empty(t, obs)
}
