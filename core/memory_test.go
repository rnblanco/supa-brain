package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemember_Insert_NoTopicKey(t *testing.T) {
	embedder := &mockEmbedder{vector: make([]float32, 768)}
	store := &mockStore{returnID: 42}
	svc := NewMemoryService(embedder, store)

	id, err := svc.Remember(context.Background(), RememberInput{
		Title:   "test title",
		Content: "**What**: test content",
		Type:    "decision",
		Project: "vettrack-pro",
		Scope:   "project",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
	assert.Len(t, store.inserted, 1)
	assert.Empty(t, store.upserted)
	assert.Equal(t, "test title", store.inserted[0].Title)
	assert.Len(t, store.inserted[0].Embedding, 768)
}

func TestRemember_Upsert_WithTopicKey(t *testing.T) {
	embedder := &mockEmbedder{vector: make([]float32, 768)}
	store := &mockStore{returnID: 1}
	svc := NewMemoryService(embedder, store)

	tk := "architecture/auth"
	_, err := svc.Remember(context.Background(), RememberInput{
		Title:    "auth decision",
		Content:  "**What**: used JWT",
		Type:     "decision",
		Project:  "vettrack-pro",
		Scope:    "project",
		TopicKey: &tk,
	})

	require.NoError(t, err)
	assert.Empty(t, store.inserted)
	assert.Len(t, store.upserted, 1)
	assert.Equal(t, &tk, store.upserted[0].TopicKey)
}

func TestRemember_EmptyContent_Error(t *testing.T) {
	svc := NewMemoryService(&mockEmbedder{}, &mockStore{})
	_, err := svc.Remember(context.Background(), RememberInput{
		Title:   "test",
		Content: "",
		Type:    "decision",
	})
	assert.ErrorContains(t, err, "content cannot be empty")
}

func TestRemember_EmbedFails_Error(t *testing.T) {
	embedder := &mockEmbedder{err: errors.New("ollama down")}
	svc := NewMemoryService(embedder, &mockStore{})
	_, err := svc.Remember(context.Background(), RememberInput{
		Title:   "test",
		Content: "some content",
		Type:    "decision",
	})
	assert.ErrorContains(t, err, "embedding service unavailable")
}

func TestRecall_EmptyQuery_Error(t *testing.T) {
	svc := NewMemoryService(&mockEmbedder{}, &mockStore{})
	_, err := svc.Recall(context.Background(), RecallInput{Query: ""})
	assert.ErrorContains(t, err, "query cannot be empty")
}

func TestRecall_EmptyQueryWithSpaces_Error(t *testing.T) {
	svc := NewMemoryService(&mockEmbedder{}, &mockStore{})
	_, err := svc.Recall(context.Background(), RecallInput{Query: "   "})
	assert.ErrorContains(t, err, "query cannot be empty")
}

func TestRecall_EmbedFails_Error(t *testing.T) {
	embedder := &mockEmbedder{err: errors.New("ollama down")}
	svc := NewMemoryService(embedder, &mockStore{})
	_, err := svc.Recall(context.Background(), RecallInput{Query: "something"})
	assert.Error(t, err)
}

func TestRecall_LimitCapped(t *testing.T) {
	embedder := &mockEmbedder{vector: make([]float32, 768)}
	store := &mockStore{}
	svc := NewMemoryService(embedder, store)

	// Limit > 50 should be capped
	_, err := svc.Recall(context.Background(), RecallInput{Query: "test", Limit: 100})
	require.NoError(t, err)
}

func TestCloseSession_SavesBoth(t *testing.T) {
	embedder := &mockEmbedder{vector: make([]float32, 768)}
	store := &mockStore{}
	svc := NewMemoryService(embedder, store)

	err := svc.CloseSession(context.Background(), CloseSessionInput{
		Project: "vettrack-pro",
		Summary: "## Goal\nBuild X\n## Accomplished\n- Done",
	})

	require.NoError(t, err)
	assert.Len(t, store.sessions, 1)
	assert.Equal(t, "vettrack-pro", store.sessions[0].Project)
}

func TestForget_CallsDelete(t *testing.T) {
	store := &mockStore{}
	svc := NewMemoryService(&mockEmbedder{}, store)

	err := svc.Forget(context.Background(), "vettrack-pro", "architecture/auth")
	assert.NoError(t, err)
}
