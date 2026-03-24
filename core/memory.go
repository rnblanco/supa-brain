package core

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MemoryService is the central business logic layer.
type MemoryService struct {
	embedder EmbedProvider
	store    MemoryStore
}

func NewMemoryService(embedder EmbedProvider, store MemoryStore) *MemoryService {
	return &MemoryService{embedder: embedder, store: store}
}

// RememberInput holds parameters for the remember MCP tool.
type RememberInput struct {
	Title    string
	Content  string
	Type     string
	Project  string
	Scope    string
	TopicKey *string
}

// Remember embeds title+content and saves or upserts the memory.
func (s *MemoryService) Remember(ctx context.Context, in RememberInput) (int64, error) {
	if !utf8.ValidString(in.Content) {
		return 0, fmt.Errorf("content must be valid UTF-8")
	}
	if strings.TrimSpace(in.Content) == "" {
		return 0, fmt.Errorf("content cannot be empty")
	}

	vec, err := s.embedder.Embed(ctx, in.Title+"\n"+in.Content)
	if err != nil {
		return 0, fmt.Errorf("embedding service unavailable; memory not saved: %w", err)
	}

	scope := in.Scope
	if scope == "" {
		scope = "project"
	}

	now := time.Now().UTC()
	m := Memory{
		Title:     in.Title,
		Content:   in.Content,
		Type:      in.Type,
		Project:   in.Project,
		Scope:     scope,
		TopicKey:  in.TopicKey,
		Embedding: vec,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if in.TopicKey != nil {
		return s.store.Upsert(ctx, m)
	}
	return s.store.Insert(ctx, m)
}

// RecallInput holds parameters for the recall MCP tool.
type RecallInput struct {
	Query   string
	Project *string
	Limit   int
}

// Recall embeds the query and returns semantically similar memories.
func (s *MemoryService) Recall(ctx context.Context, in RecallInput) ([]MemoryResult, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	vec, err := s.embedder.Embed(ctx, in.Query)
	if err != nil {
		return nil, fmt.Errorf("embedding service unavailable: %w", err)
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	return s.store.Search(ctx, SearchQuery{
		Embedding:     vec,
		Project:       in.Project,
		Limit:         limit,
		MinSimilarity: 0.3,
	})
}

// CloseSessionInput holds parameters for the close_session MCP tool.
type CloseSessionInput struct {
	Project string
	Summary string
}

// CloseSession embeds the summary and saves it to both sessions and memories atomically.
func (s *MemoryService) CloseSession(ctx context.Context, in CloseSessionInput) error {
	vec, err := s.embedder.Embed(ctx, in.Summary)
	if err != nil {
		return fmt.Errorf("embedding service unavailable; session not saved: %w", err)
	}

	session := Session{
		ID:      fmt.Sprintf("sess-%s-%s", in.Project, uuid.New().String()[:8]),
		Project: in.Project,
		Summary: in.Summary,
	}

	return s.store.SaveSession(ctx, session, vec)
}

// GetByID returns the full memory for the given ID.
func (s *MemoryService) GetByID(ctx context.Context, id int64) (*Memory, error) {
	m, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("memory store unreachable; operation failed: %w", err)
	}
	return m, nil
}

// Forget deletes the memory matching (project, topicKey).
func (s *MemoryService) Forget(ctx context.Context, project, topicKey string) error {
	return s.store.Delete(ctx, project, topicKey)
}

// UpdateInput holds parameters for the mem_update MCP tool.
type UpdateInput struct {
	ID       int64
	Title    *string
	Content  *string
	Type     *string
	Scope    *string
	TopicKey *string
}

// Update patches an existing observation by ID. Re-embeds if title or content change.
func (s *MemoryService) Update(ctx context.Context, in UpdateInput) error {
	if in.Title == nil && in.Content == nil && in.Type == nil &&
		in.Scope == nil && in.TopicKey == nil {
		return fmt.Errorf("at least one field must be provided")
	}

	fields := UpdateFields{
		Title:    in.Title,
		Content:  in.Content,
		Type:     in.Type,
		Scope:    in.Scope,
		TopicKey: in.TopicKey,
	}

	// Re-embed only if title or content changed — both contribute to the vector.
	if in.Title != nil || in.Content != nil {
		current, err := s.store.GetByID(ctx, in.ID)
		if err != nil {
			return fmt.Errorf("cannot fetch memory %d: %w", in.ID, err)
		}
		if current == nil {
			return fmt.Errorf("memory %d not found", in.ID)
		}

		title := current.Title
		if in.Title != nil {
			title = *in.Title
		}
		content := current.Content
		if in.Content != nil {
			content = *in.Content
		}

		vec, err := s.embedder.Embed(ctx, title+"\n"+content)
		if err != nil {
			return fmt.Errorf("embedding service unavailable: %w", err)
		}
		fields.Embedding = vec
	}

	return s.store.UpdateByID(ctx, in.ID, fields)
}

// GetContext returns recent sessions and observations for session recovery.
func (s *MemoryService) GetContext(ctx context.Context, project string, limit int) (*ContextResult, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.store.GetRecentContext(ctx, project, limit)
}
