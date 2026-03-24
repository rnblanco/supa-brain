package supabase

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"memory-server/core"
)

func getTestStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_KEY")
	if url == "" || key == "" {
		t.Skip("SUPABASE_URL and SUPABASE_KEY not set — skipping integration test")
	}
	store, err := New(context.Background(), url, key, 5, 15*time.Second)
	require.NoError(t, err)
	return store
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
	assert.Equal(t, "test-integration", got.Project)

	// Cleanup — delete all for this project
	_ = store.Delete(ctx, "test-integration", "")
}

func TestUpsert_WithTopicKey(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	tk := "test/upsert-key-integration"
	m := core.Memory{
		Title:     "original title",
		Content:   "original content",
		Type:      "decision",
		Project:   "test-integration",
		Scope:     "project",
		TopicKey:  &tk,
		Embedding: make([]float32, 768),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	id1, err := store.Upsert(ctx, m)
	require.NoError(t, err)
	assert.Greater(t, id1, int64(0))

	m.Title = "updated title"
	id2, err := store.Upsert(ctx, m)
	require.NoError(t, err)

	// Same row updated — IDs should match
	assert.Equal(t, id1, id2)

	got, err := store.GetByID(ctx, id1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "updated title", got.Title)

	// Cleanup
	_ = store.Delete(ctx, "test-integration", tk)
}

func TestSearch_ReturnsResults(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	// Insert a memory with zero embedding
	m := core.Memory{
		Title:     "searchable test memory",
		Content:   "**What**: test\n**Why**: search test",
		Type:      "decision",
		Project:   "test-integration",
		Scope:     "project",
		Embedding: make([]float32, 768),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := store.Insert(ctx, m)
	require.NoError(t, err)

	// Search with zero vector — should find our inserted memory
	results, err := store.Search(ctx, core.SearchQuery{
		Embedding:     make([]float32, 768),
		Limit:         5,
		MinSimilarity: 0.0,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, results)

	// Cleanup
	_ = store.Delete(ctx, "test-integration", "")
}

func TestSaveSession_Atomic(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	session := core.Session{
		ID:      "test-session-atomic-001",
		Project: "test-integration",
		Summary: "## Goal\nTest atomic session save\n## Accomplished\n- Done",
	}

	err := store.SaveSession(ctx, session, make([]float32, 768))
	require.NoError(t, err)

	// Cleanup — delete session memory
	_ = store.Delete(ctx, "test-integration", "")
}

func TestDelete_ByTopicKey(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	tk := "test/delete-by-topic"
	m := core.Memory{
		Title:     "to be deleted",
		Content:   "delete me",
		Type:      "decision",
		Project:   "test-integration",
		Scope:     "project",
		TopicKey:  &tk,
		Embedding: make([]float32, 768),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	id, err := store.Insert(ctx, m)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	err = store.Delete(ctx, "test-integration", tk)
	require.NoError(t, err)

	got, err := store.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, got, "memory should be gone after delete")
}

func TestExport_ReturnsMemories(t *testing.T) {
	store := getTestStore(t)
	ctx := context.Background()

	project := "test-export-project"
	m := core.Memory{
		Title:     "export test memory",
		Content:   "export content",
		Type:      "discovery",
		Project:   project,
		Scope:     "project",
		Embedding: make([]float32, 768),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := store.Insert(ctx, m)
	require.NoError(t, err)

	results, err := store.Export(ctx, core.ExportFilter{Project: &project})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Equal(t, project, results[0].Project)

	// Cleanup
	_ = store.Delete(ctx, project, "")
}
