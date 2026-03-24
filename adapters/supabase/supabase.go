package supabase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	"supa-brain/core"
)

// Store implements core.MemoryStore using Supabase (PostgreSQL + pgvector).
type Store struct {
	pool *pgxpool.Pool
}

// New creates a pgxpool connection to Supabase.
//
// Connection is resolved in priority order:
//  1. dbURL — if non-empty, used directly as a PostgreSQL DSN.
//     Get it from Supabase Dashboard → Settings → Database → Connection string (URI mode).
//     Set it as DB_URL in ~/.supa-brain/config.env.
//  2. serviceKey — if it starts with "postgresql://" or "postgres://", used as DSN.
//     This is a backward-compatible fallback for existing configs that set SUPABASE_KEY
//     to a full connection string.
//  3. Otherwise returns an error with setup instructions.
func New(ctx context.Context, dbURL, serviceKey string, maxConns int, connectTimeout time.Duration) (*Store, error) {
	switch {
	case dbURL != "":
		return newFromDSN(ctx, dbURL, maxConns, connectTimeout)
	case strings.HasPrefix(serviceKey, "postgresql://") || strings.HasPrefix(serviceKey, "postgres://"):
		return newFromDSN(ctx, serviceKey, maxConns, connectTimeout)
	default:
		return nil, fmt.Errorf(
			"DB_URL is not set.\n" +
				"Get the connection string from Supabase Dashboard → Settings → Database → Connection string (URI mode)\n" +
				"and add DB_URL=postgresql://... to ~/.supa-brain/config.env",
		)
	}
}

// NewFromDSN creates a Store from a full PostgreSQL connection string.
// This is useful when the caller has the direct DSN from the Supabase dashboard.
func NewFromDSN(ctx context.Context, dsn string, maxConns int, connectTimeout time.Duration) (*Store, error) {
	return newFromDSN(ctx, dsn, maxConns, connectTimeout)
}

func newFromDSN(ctx context.Context, dsn string, maxConns int, connectTimeout time.Duration) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("supabase: parse config: %w", err)
	}

	cfg.MaxConns = int32(maxConns)
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.ConnConfig.ConnectTimeout = connectTimeout

	// pgvector requires type registration on every new connection.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("supabase: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("supabase: ping failed: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Insert always creates a new row, regardless of topic_key.
func (s *Store) Insert(ctx context.Context, m core.Memory) (int64, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = time.Now()
	}

	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO memories (title, content, type, project, scope, topic_key, embedding, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		m.Title, m.Content, m.Type, m.Project, m.Scope,
		m.TopicKey, pgvector.NewVector(m.Embedding),
		m.CreatedAt, m.UpdatedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("supabase: insert: %w", err)
	}
	return id, nil
}

// Upsert updates the existing row matching (project, topic_key) if found,
// otherwise inserts. Falls back to Insert when TopicKey is nil.
func (s *Store) Upsert(ctx context.Context, m core.Memory) (int64, error) {
	if m.TopicKey == nil {
		return s.Insert(ctx, m)
	}

	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO memories (title, content, type, project, scope, topic_key, embedding, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())
		ON CONFLICT (project, topic_key) WHERE topic_key IS NOT NULL
		DO UPDATE SET
			title      = EXCLUDED.title,
			content    = EXCLUDED.content,
			type       = EXCLUDED.type,
			scope      = EXCLUDED.scope,
			embedding  = EXCLUDED.embedding,
			updated_at = now()
		RETURNING id`,
		m.Title, m.Content, m.Type, m.Project, m.Scope,
		m.TopicKey, pgvector.NewVector(m.Embedding),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("supabase: upsert: %w", err)
	}
	return id, nil
}

// Search runs a cosine-similarity query via the search_memories SQL function.
func (s *Store) Search(ctx context.Context, q core.SearchQuery) ([]core.MemoryResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	minSim := q.MinSimilarity
	if minSim < 0 {
		minSim = 0.3
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, title, content, type, project, scope, topic_key, similarity, created_at
		 FROM search_memories($1, $2, $3, $4, $5)`,
		pgvector.NewVector(q.Embedding),
		q.Project, q.Scope, limit, minSim,
	)
	if err != nil {
		return nil, fmt.Errorf("supabase: search: %w", err)
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		if err := rows.Scan(
			&r.ID, &r.Title, &r.Content, &r.Type,
			&r.Project, &r.Scope, &r.TopicKey,
			&r.Similarity, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("supabase: scan result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("supabase: rows error: %w", err)
	}
	return results, nil
}

// GetByID returns the full memory for the given ID, or nil if not found.
func (s *Store) GetByID(ctx context.Context, id int64) (*core.Memory, error) {
	var m core.Memory
	err := s.pool.QueryRow(ctx,
		`SELECT id, title, content, type, project, scope, topic_key, created_at, updated_at
		 FROM memories WHERE id = $1`, id,
	).Scan(
		&m.ID, &m.Title, &m.Content, &m.Type,
		&m.Project, &m.Scope, &m.TopicKey,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("supabase: get by id: %w", err)
	}
	return &m, nil
}

// Delete removes memories matching (project, topicKey).
// If topicKey is empty, deletes ALL memories for the project (useful for test cleanup).
func (s *Store) Delete(ctx context.Context, project, topicKey string) error {
	var err error
	if topicKey == "" {
		_, err = s.pool.Exec(ctx,
			`DELETE FROM memories WHERE project = $1`, project)
	} else {
		_, err = s.pool.Exec(ctx,
			`DELETE FROM memories WHERE project = $1 AND topic_key = $2`,
			project, topicKey)
	}
	if err != nil {
		return fmt.Errorf("supabase: delete: %w", err)
	}
	return nil
}

// SaveSession writes to the sessions table AND inserts a session_summary memory
// atomically (single transaction). If the session ID already exists it is updated.
func (s *Store) SaveSession(ctx context.Context, sess core.Session, embedding []float32) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("supabase: begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO sessions (id, project, summary, started_at, ended_at)
		VALUES ($1, $2, $3, now(), now())
		ON CONFLICT (id)
		DO UPDATE SET
			summary  = EXCLUDED.summary,
			ended_at = now()`,
		sess.ID, sess.Project, sess.Summary,
	)
	if err != nil {
		return fmt.Errorf("supabase: save session record: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO memories (title, content, type, project, scope, embedding, created_at, updated_at)
		VALUES ($1, $2, 'session_summary', $3, 'project', $4, now(), now())`,
		fmt.Sprintf("Session: %s", sess.Project),
		sess.Summary,
		sess.Project,
		pgvector.NewVector(embedding),
	)
	if err != nil {
		return fmt.Errorf("supabase: save session memory: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("supabase: commit session: %w", err)
	}
	return nil
}

// UpdateByID patches an existing memory. Only non-nil fields in UpdateFields are changed.
func (s *Store) UpdateByID(ctx context.Context, id int64, fields core.UpdateFields) error {
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if fields.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *fields.Title)
		argIdx++
	}
	if fields.Content != nil {
		setClauses = append(setClauses, fmt.Sprintf("content = $%d", argIdx))
		args = append(args, *fields.Content)
		argIdx++
	}
	if fields.Type != nil {
		setClauses = append(setClauses, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *fields.Type)
		argIdx++
	}
	if fields.Scope != nil {
		setClauses = append(setClauses, fmt.Sprintf("scope = $%d", argIdx))
		args = append(args, *fields.Scope)
		argIdx++
	}
	if fields.TopicKey != nil {
		setClauses = append(setClauses, fmt.Sprintf("topic_key = $%d", argIdx))
		if *fields.TopicKey == "" {
			args = append(args, nil) // clear to NULL
		} else {
			args = append(args, *fields.TopicKey)
		}
		argIdx++
	}
	if fields.Embedding != nil {
		setClauses = append(setClauses, fmt.Sprintf("embedding = $%d", argIdx))
		args = append(args, pgvector.NewVector(fields.Embedding))
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now().UTC())
	argIdx++

	args = append(args, id)
	query := fmt.Sprintf("UPDATE memories SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argIdx)

	_, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("supabase: update by id: %w", err)
	}
	return nil
}

// GetRecentContext returns recent sessions and observations ordered by recency.
func (s *Store) GetRecentContext(ctx context.Context, project string, limit int) (*core.ContextResult, error) {
	result := &core.ContextResult{}

	// Recent sessions
	var (
		sessionRows pgx.Rows
		err         error
	)
	if project != "" {
		sessionRows, err = s.pool.Query(ctx,
			`SELECT id, project, summary, started_at, ended_at FROM sessions
			 WHERE project = $1 ORDER BY started_at DESC LIMIT $2`,
			project, limit)
	} else {
		sessionRows, err = s.pool.Query(ctx,
			`SELECT id, project, summary, started_at, ended_at FROM sessions
			 ORDER BY started_at DESC LIMIT $1`,
			limit)
	}
	if err != nil {
		return nil, fmt.Errorf("supabase: context sessions: %w", err)
	}
	for sessionRows.Next() {
		var sess core.Session
		if err := sessionRows.Scan(&sess.ID, &sess.Project, &sess.Summary, &sess.StartedAt, &sess.EndedAt); err != nil {
			sessionRows.Close()
			return nil, fmt.Errorf("supabase: scan session: %w", err)
		}
		result.Sessions = append(result.Sessions, sess)
	}
	if err := sessionRows.Err(); err != nil {
		return nil, fmt.Errorf("supabase: session rows: %w", err)
	}
	sessionRows.Close()

	// Recent observations
	var obsRows pgx.Rows
	if project != "" {
		obsRows, err = s.pool.Query(ctx,
			`SELECT id, title, content, type, project, scope, topic_key, created_at, updated_at
			 FROM memories WHERE project = $1 ORDER BY created_at DESC LIMIT $2`,
			project, limit)
	} else {
		obsRows, err = s.pool.Query(ctx,
			`SELECT id, title, content, type, project, scope, topic_key, created_at, updated_at
			 FROM memories ORDER BY created_at DESC LIMIT $1`,
			limit)
	}
	if err != nil {
		return nil, fmt.Errorf("supabase: context observations: %w", err)
	}
	defer obsRows.Close()

	for obsRows.Next() {
		var m core.Memory
		if err := obsRows.Scan(
			&m.ID, &m.Title, &m.Content, &m.Type,
			&m.Project, &m.Scope, &m.TopicKey,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("supabase: scan observation: %w", err)
		}
		result.Observations = append(result.Observations, m)
	}
	if err := obsRows.Err(); err != nil {
		return nil, fmt.Errorf("supabase: observation rows: %w", err)
	}

	return result, nil
}

// Export returns all memories matching the filter, ordered by created_at ASC.
func (s *Store) Export(ctx context.Context, f core.ExportFilter) ([]core.Memory, error) {
	var (
		query string
		args  []interface{}
	)

	if f.Project != nil {
		query = `
			SELECT id, title, content, type, project, scope, topic_key, created_at, updated_at
			FROM memories
			WHERE project = $1
			ORDER BY created_at ASC`
		args = []interface{}{*f.Project}
	} else {
		query = `
			SELECT id, title, content, type, project, scope, topic_key, created_at, updated_at
			FROM memories
			ORDER BY created_at ASC`
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("supabase: export: %w", err)
	}
	defer rows.Close()

	var memories []core.Memory
	for rows.Next() {
		var m core.Memory
		if err := rows.Scan(
			&m.ID, &m.Title, &m.Content, &m.Type,
			&m.Project, &m.Scope, &m.TopicKey,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("supabase: export scan: %w", err)
		}
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("supabase: export rows: %w", err)
	}
	return memories, nil
}
