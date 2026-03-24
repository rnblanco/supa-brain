package core

import (
	"context"
	"io"
)

// EmbedProvider generates a 768-dim vector embedding for the given text.
// Returns error if the provider is unreachable or times out.
type EmbedProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// MemoryStore persists and retrieves memories from Supabase.
type MemoryStore interface {
	// Insert always creates a new row.
	Insert(ctx context.Context, m Memory) (int64, error)

	// Upsert updates the existing row matching (project, topic_key) if found,
	// otherwise inserts. If m.TopicKey is nil, behaves like Insert.
	// On update: all fields are replaced except id and created_at.
	Upsert(ctx context.Context, m Memory) (int64, error)

	// Search runs a cosine similarity query via the search_memories SQL function.
	Search(ctx context.Context, q SearchQuery) ([]MemoryResult, error)

	// GetByID returns the full memory for the given ID, or nil if not found.
	GetByID(ctx context.Context, id int64) (*Memory, error)

	// Delete removes the memory matching (project, topicKey).
	Delete(ctx context.Context, project, topicKey string) error

	// SaveSession writes to the sessions table AND inserts a session_summary
	// memory atomically (single transaction). Embedding must be pre-computed.
	SaveSession(ctx context.Context, s Session, embedding []float32) error

	// Export returns all memories matching the filter.
	Export(ctx context.Context, f ExportFilter) ([]Memory, error)
}

// Exporter serializes a slice of memories to an output stream.
type Exporter interface {
	Export(ctx context.Context, w io.Writer, memories []Memory) error
}
