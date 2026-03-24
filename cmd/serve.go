package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/mcp"

	"supa-brain/adapters/ollama"
	"supa-brain/adapters/supabase"
	"supa-brain/core"
	"supa-brain/internal/config"
)

func runStdio(_ []string) error {
	cfgPath := filepath.Join(os.Getenv("USERPROFILE"), ".supa-brain", "config.env")
	cfg := config.LoadConfig(cfgPath)

	if cfg.SupabaseURL == "" {
		return fmt.Errorf("SUPABASE_URL not set in %s", cfgPath)
	}

	ctx := context.Background()

	// Ollama health check (3 retries × 2 s)
	ollamaClient := ollama.New(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaTimeout)
	var healthErr error
	for i := 0; i < 3; i++ {
		healthErr = ollamaClient.CheckHealth(ctx)
		if healthErr == nil {
			break
		}
		if i < 2 {
			time.Sleep(2 * time.Second)
		}
	}
	if healthErr != nil {
		return fmt.Errorf("ollama not reachable at %s — start Ollama before supa-brain", cfg.OllamaURL)
	}

	// Supabase store
	store, err := supabase.New(ctx, cfg.SupabaseURL, cfg.SupabaseKey, cfg.DBMaxConns, cfg.DBConnectTimeout)
	if err != nil {
		return fmt.Errorf("cannot connect to Supabase: %w", err)
	}

	svc := core.NewMemoryService(ollamaClient, store)

	s := mcpserver.NewMCPServer("supa-brain", "1.0.0")

	// ── Tool: mem_save ─────────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_save",
		mcp.WithDescription("Save or upsert a memory observation. Use topic_key to update an existing topic."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Short searchable title (Verb + what)")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Full observation content")),
		mcp.WithString("type", mcp.Required(), mcp.Description("One of: bugfix | decision | architecture | discovery | pattern | config | preference")),
		mcp.WithString("project", mcp.Description("Project slug (inferred from CWD if omitted)")),
		mcp.WithString("scope", mcp.Description("project (default) or personal")),
		mcp.WithString("topic_key", mcp.Description("Stable key for upsert, e.g. architecture/auth-model")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := req.GetString("title", "")
		content := req.GetString("content", "")
		memType := req.GetString("type", "")
		project := req.GetString("project", inferProject(cfg.MemoryProject))
		scope := req.GetString("scope", "project")
		topicKeyStr := req.GetString("topic_key", "")

		if title == "" || content == "" || memType == "" {
			return mcp.NewToolResultError("title, content, and type are required"), nil
		}

		var topicKey *string
		if topicKeyStr != "" {
			topicKey = &topicKeyStr
		}

		id, err := svc.Remember(ctx, core.RememberInput{
			Title:    title,
			Content:  content,
			Type:     memType,
			Project:  project,
			Scope:    scope,
			TopicKey: topicKey,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("remember failed: %s", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"id":%d,"project":%q}`, id, project)), nil
	})

	// ── Tool: mem_search ───────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_search",
		mcp.WithDescription("Search memories by semantic similarity."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Natural language search query")),
		mcp.WithString("project", mcp.Description("Limit to a project slug (omit for all projects)")),
		mcp.WithNumber("limit", mcp.Description("Max results (1–50, default 10)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}

		var projectPtr *string
		if p := req.GetString("project", ""); p != "" {
			projectPtr = &p
		}

		limit := 10
		if args := req.GetArguments(); args != nil {
			if v, ok := args["limit"]; ok {
				switch n := v.(type) {
				case float64:
					limit = int(n)
				case int:
					limit = n
				}
			}
		}

		results, err := svc.Recall(ctx, core.RecallInput{
			Query:   query,
			Project: projectPtr,
			Limit:   limit,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("recall failed: %s", err)), nil
		}

		out, _ := json.Marshal(results)
		return mcp.NewToolResultText(string(out)), nil
	})

	// ── Tool: mem_session_summary ──────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_session_summary",
		mcp.WithDescription("Save an end-of-session summary to persistent memory."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project slug")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Full session summary content")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		summary := req.GetString("summary", "")

		if project == "" || summary == "" {
			return mcp.NewToolResultError("project and summary are required"), nil
		}

		if err := svc.CloseSession(ctx, core.CloseSessionInput{
			Project: project,
			Summary: summary,
		}); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("close_session failed: %s", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"status":"ok","project":%q}`, project)), nil
	})

	// ── Tool: mem_get_observation ──────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_get_observation",
		mcp.WithDescription("Retrieve a specific memory by its numeric ID."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Numeric memory ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			return mcp.NewToolResultError("id is required"), nil
		}
		v, ok := args["id"]
		if !ok {
			return mcp.NewToolResultError("id is required"), nil
		}
		var id int64
		switch n := v.(type) {
		case float64:
			id = int64(n)
		case int:
			id = int64(n)
		case int64:
			id = n
		default:
			return mcp.NewToolResultError("id must be a number"), nil
		}

		mem, err := svc.GetByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get_memory failed: %s", err)), nil
		}
		if mem == nil {
			return mcp.NewToolResultError(fmt.Sprintf("memory %d not found", id)), nil
		}
		out, _ := json.Marshal(mem)
		return mcp.NewToolResultText(string(out)), nil
	})

	// ── Tool: mem_update ───────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_update",
		mcp.WithDescription("Update an existing observation by ID. Only provided fields are changed. Re-embeds if title or content changes."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Numeric observation ID to update")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("content", mcp.Description("New content")),
		mcp.WithString("type", mcp.Description("New type: bugfix | decision | architecture | discovery | pattern | config | preference")),
		mcp.WithString("scope", mcp.Description("New scope: project or personal")),
		mcp.WithString("topic_key", mcp.Description("New topic key. Empty string clears it to NULL.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			return mcp.NewToolResultError("id is required"), nil
		}
		v, ok := args["id"]
		if !ok {
			return mcp.NewToolResultError("id is required"), nil
		}
		var id int64
		switch n := v.(type) {
		case float64:
			id = int64(n)
		case int64:
			id = n
		default:
			return mcp.NewToolResultError("id must be a number"), nil
		}

		in := core.UpdateInput{ID: id}
		if s := req.GetString("title", ""); s != "" {
			in.Title = &s
		}
		if s := req.GetString("content", ""); s != "" {
			in.Content = &s
		}
		if s := req.GetString("type", ""); s != "" {
			in.Type = &s
		}
		if s := req.GetString("scope", ""); s != "" {
			in.Scope = &s
		}
		// topic_key: present in args = update (even if empty string = clear to NULL)
		if _, hasTK := args["topic_key"]; hasTK {
			s := req.GetString("topic_key", "")
			in.TopicKey = &s
		}

		if err := svc.Update(ctx, in); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update failed: %s", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"status":"updated","id":%d}`, id)), nil
	})

	// ── Tool: mem_delete ───────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_delete",
		mcp.WithDescription("Delete the memory matching a project + topic_key pair."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project slug")),
		mcp.WithString("topic_key", mcp.Required(), mcp.Description("Topic key of the memory to delete")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		topicKey := req.GetString("topic_key", "")

		if project == "" || topicKey == "" {
			return mcp.NewToolResultError("project and topic_key are required"), nil
		}

		if err := svc.Forget(ctx, project, topicKey); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("delete failed: %s", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"status":"deleted","project":%q,"topic_key":%q}`, project, topicKey)), nil
	})

	// ── Tool: mem_context ──────────────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_context",
		mcp.WithDescription("Get recent memory context from previous sessions. Returns recent sessions and observations ordered by recency — no embedding cost."),
		mcp.WithString("project", mcp.Description("Filter by project slug (omit for all projects)")),
		mcp.WithNumber("limit", mcp.Description("Number of items to retrieve per category (default 10)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project := req.GetString("project", "")
		limit := 10
		if args := req.GetArguments(); args != nil {
			if v, ok := args["limit"]; ok {
				if n, ok := v.(float64); ok {
					limit = int(n)
				}
			}
		}

		result, err := svc.GetContext(ctx, project, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("context failed: %s", err)), nil
		}
		return mcp.NewToolResultText(formatContext(result, project)), nil
	})

	// ── Tool: mem_suggest_topic_key ────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_suggest_topic_key",
		mcp.WithDescription("Suggest a stable topic_key for memory upserts. Use before mem_save when you want evolving topics to update a single observation over time."),
		mcp.WithString("title", mcp.Description("Observation title (preferred input)")),
		mcp.WithString("type", mcp.Description("Observation type, e.g. architecture, decision, bugfix")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title := req.GetString("title", "")
		memType := req.GetString("type", "")
		key := suggestTopicKey(title, memType)
		return mcp.NewToolResultText(fmt.Sprintf(`{"topic_key":%q}`, key)), nil
	})

	// ── Tool: mem_capture_passive ──────────────────────────────────────────────
	s.AddTool(mcp.NewTool("mem_capture_passive",
		mcp.WithDescription("Extract and save structured learnings from text output. Looks for '## Key Learnings:' or '## Aprendizajes Clave:' sections and saves each numbered or bulleted item as a separate observation."),
		mcp.WithString("content", mcp.Required(), mcp.Description("Text containing a '## Key Learnings:' section")),
		mcp.WithString("project", mcp.Description("Project slug (inferred from CWD if omitted)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		content := req.GetString("content", "")
		if content == "" {
			return mcp.NewToolResultError("content is required"), nil
		}
		project := req.GetString("project", inferProject(cfg.MemoryProject))

		items := extractLearnings(content)
		saved := 0
		for _, item := range items {
			title := item
			if len(title) > 80 {
				title = title[:80]
			}
			_, err := svc.Remember(ctx, core.RememberInput{
				Title:   title,
				Content: item,
				Type:    "discovery",
				Project: project,
				Scope:   "project",
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("capture failed at item %d: %s", saved+1, err)), nil
			}
			saved++
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"extracted":%d,"saved":%d,"duplicates":0}`, len(items), saved)), nil
	})

	return mcpserver.ServeStdio(s)
}

// formatContext renders a ContextResult as human-readable markdown for the AI.
func formatContext(result *core.ContextResult, project string) string {
	var sb strings.Builder

	header := "## Memory Context"
	if project != "" {
		header += " — " + project
	}
	sb.WriteString(header + "\n\n")

	sb.WriteString("### Recent Sessions\n")
	if len(result.Sessions) == 0 {
		sb.WriteString("No sessions found.\n")
	} else {
		for _, sess := range result.Sessions {
			preview := sess.Summary
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			sb.WriteString(fmt.Sprintf("- **%s** | %s | %s\n",
				sess.Project, sess.StartedAt.Format("2006-01-02 15:04"), preview))
		}
	}

	sb.WriteString("\n### Recent Observations\n")
	if len(result.Observations) == 0 {
		sb.WriteString("No observations found.\n")
	} else {
		for _, m := range result.Observations {
			preview := m.Content
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("- [%s] **%s** | %s\n  %s\n",
				m.Type, m.Title, m.CreatedAt.Format("2006-01-02 15:04"), preview))
		}
	}

	return sb.String()
}

// suggestTopicKey normalizes a title and optional type into a stable slug.
// e.g. "Fixed JWT auth middleware", "bugfix" → "bugfix/fixed-jwt-auth-middleware"
func suggestTopicKey(title, memType string) string {
	key := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	// collapse multiple dashes
	parts := strings.FieldsFunc(b.String(), func(r rune) bool { return r == '-' })
	slug := strings.Join(parts, "-")

	if memType != "" {
		return strings.ToLower(memType) + "/" + slug
	}
	return slug
}

// extractLearnings parses "## Key Learnings:" or "## Aprendizajes Clave:" sections
// and returns each numbered or bulleted item as a separate string.
func extractLearnings(content string) []string {
	lines := strings.Split(content, "\n")
	seen := map[string]bool{}
	var items []string
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "## key learnings") || strings.HasPrefix(lower, "## aprendizajes") {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, "##") {
			break
		}
		if !inSection {
			continue
		}

		item := ""
		switch {
		case strings.HasPrefix(trimmed, "- "):
			item = strings.TrimSpace(trimmed[2:])
		case strings.HasPrefix(trimmed, "* "):
			item = strings.TrimSpace(trimmed[2:])
		case len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9':
			if idx := strings.IndexAny(trimmed, ".)"); idx > 0 && idx < 4 {
				item = strings.TrimSpace(trimmed[idx+1:])
			}
		}

		if item != "" && !seen[item] {
			seen[item] = true
			items = append(items, item)
		}
	}
	return items
}
