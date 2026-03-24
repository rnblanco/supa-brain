package supabase

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"memory-server/core"
)

var (
	sharedStore *Store
	storeOnce   sync.Once
	storeErr    error
)

func getTestStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_KEY")
	if url == "" || key == "" {
		t.Skip("SUPABASE_URL and SUPABASE_KEY not set — skipping integration test")
	}

	storeOnce.Do(func() {
		sharedStore, storeErr = New(context.Background(), url, key, 3, 10*time.Second)
	})

	require.NoError(t, storeErr)
	return sharedStore
}

func TestInsert_And_GetByID(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	m := core.Memory{
		Title:     "test memory integration",
		Content:   "**What**: integration test\n**Why**: verify Supabase adapter",
		Type:      "decision",
		Project:   "test-integration",
		Scope:     "project",
		Embedding: make([]float32, 768),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	id, err := store.Insert(ctx, m)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	got, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "test memory integration", got.Title)
}

func TestUpsert_WithTopicKey(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	tk := "test/upsert-key-integration"
	m := core.Memory{
		Title:    "original title",
		Content:  "original content",
		Type:     "decision",
		Project:  "test-integration",
		Scope:    "project",
		TopicKey: &tk,
		Embedding: make([]float32, 768),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	id1, err := store.Upsert(ctx, m)
	require.NoError(t, err)

	m.Title = "updated title"
	id2, err := store.Upsert(ctx, m)
	require.NoError(t, err)
	assert.Equal(t, id1, id2)

	got, err := store.GetByID(ctx, id1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "updated title", got.Title)

	store.Delete(ctx, "test-integration", tk)
}

func TestSearch_ReturnsResults(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	// Use a simple non-zero embedding so cosine similarity is well-defined
	emb := make([]float32, 768)
	emb[0] = 1.0

	m := core.Memory{
		Title:     "searchable test memory",
		Content:   "**What**: test\n**Why**: search test",
		Type:      "decision",
		Project:   "test-integration",
		Scope:     "project",
		Embedding: emb,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	id, err := store.Insert(ctx, m)
	require.NoError(t, err)
	defer store.Delete(ctx, "test-integration", "")

	results, err := store.Search(ctx, core.SearchQuery{
		Embedding:     emb,
		Limit:         10,
		MinSimilarity: 0.1,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	_ = id
}

func TestSaveSession_Atomic(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	emb := make([]float32, 768)
	emb[0] = 1.0

	session := core.Session{
		ID:      "test-session-atomic-001",
		Project: "test-integration",
		Summary: "## Goal\nTest atomic session save\n## Accomplished\n- Done",
	}

	err := store.SaveSession(ctx, session, emb)
	require.NoError(t, err)
}

func TestExport_ReturnsMemories(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	memories, err := store.Export(ctx, core.ExportFilter{})
	require.NoError(t, err)
	// May be empty or not — just verify it doesn't error
	_ = memories
}
