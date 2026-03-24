package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/mcp"

	"memory-server/adapters/ollama"
	"memory-server/adapters/supabase"
	"memory-server/core"
	"memory-server/internal/config"
)

func runStdio(_ []string) error {
	cfgPath := filepath.Join(os.Getenv("USERPROFILE"), ".memory-server", "config.env")
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
		return fmt.Errorf("ollama not reachable at %s — start Ollama before memory-server", cfg.OllamaURL)
	}

	// Supabase store
	store, err := supabase.New(ctx, cfg.SupabaseURL, cfg.SupabaseKey, cfg.DBMaxConns, cfg.DBConnectTimeout)
	if err != nil {
		return fmt.Errorf("cannot connect to Supabase: %w", err)
	}

	svc := core.NewMemoryService(ollamaClient, store)

	s := mcpserver.NewMCPServer("memory-server", "1.0.0")

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
			return mcp.NewToolResultError(fmt.Sprintf("forget failed: %s", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(`{"status":"deleted","project":%q,"topic_key":%q}`, project, topicKey)), nil
	})

	return mcpserver.ServeStdio(s)
}
