package migration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sampleJSON = []byte(`{
  "version": "1.0",
  "exported_at": "2026-03-24T00:00:00Z",
  "observations": [
    {
      "id": 1,
      "sync_id": "obs-abc123",
      "session_id": "manual-save-vettrack-pro",
      "title": "Fixed N+1 query",
      "content": "**What**: Added eager loading\n**Why**: Performance",
      "type": "bugfix",
      "project": "vettrack-pro",
      "scope": "project",
      "topic_key": "bugs/n-plus-one",
      "revision_count": 1,
      "duplicate_count": 1,
      "created_at": "2026-03-20T07:08:10Z",
      "updated_at": "2026-03-20T07:08:10Z"
    },
    {
      "id": 2,
      "title": "No topic key",
      "content": "**What**: something",
      "type": "decision",
      "project": "personal",
      "scope": "personal",
      "topic_key": null,
      "created_at": "2026-03-21T09:00:00Z",
      "updated_at": "2026-03-21T09:00:00Z"
    }
  ],
  "sessions": [],
  "prompts": []
}`)

func TestParseEngram_Success(t *testing.T) {
	memories, err := ParseEngram(sampleJSON)

	require.NoError(t, err)
	assert.Len(t, memories, 2)

	assert.Equal(t, "Fixed N+1 query", memories[0].Title)
	assert.Equal(t, "vettrack-pro", memories[0].Project)
	assert.NotNil(t, memories[0].TopicKey)
	assert.Equal(t, "bugs/n-plus-one", *memories[0].TopicKey)

	assert.Nil(t, memories[1].TopicKey)
	assert.Equal(t, "personal", memories[1].Scope)

	expected, _ := time.Parse(time.RFC3339, "2026-03-20T07:08:10Z")
	assert.Equal(t, expected.UTC(), memories[0].CreatedAt.UTC())
}

func TestParseEngram_InvalidJSON(t *testing.T) {
	_, err := ParseEngram([]byte(`not json`))
	assert.Error(t, err)
}

func TestParseEngram_EmptyObservations(t *testing.T) {
	data := []byte(`{"version":"1.0","observations":[],"sessions":[],"prompts":[]}`)
	memories, err := ParseEngram(data)
	require.NoError(t, err)
	assert.Empty(t, memories)
}
