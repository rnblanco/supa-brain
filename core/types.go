package core

import "time"

// Memory is a single persisted observation.
type Memory struct {
	ID        int64
	Title     string
	Content   string
	Type      string    // decision | bugfix | pattern | config | discovery | session_summary
	Project   string
	Scope     string    // project | personal
	TopicKey  *string   // nil = always insert; non-nil = upsert by (project, topic_key)
	Embedding []float32 // 768-dim vector from nomic-embed-text
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Session is an end-of-session summary record (stored in sessions table).
type Session struct {
	ID        string
	Project   string
	Summary   string
	StartedAt time.Time
	EndedAt   *time.Time
}

// SearchQuery holds parameters for a semantic memory search.
type SearchQuery struct {
	Embedding     []float32
	Project       *string // nil = all projects
	Scope         *string // nil = all scopes
	Limit         int     // default 10, max 50
	MinSimilarity float64 // default 0.3
}

// MemoryResult is one search hit with its cosine similarity score.
type MemoryResult struct {
	Memory
	Similarity float64
}

// ExportFilter scopes an export operation.
type ExportFilter struct {
	Project *string // nil = all projects
}
