package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"

	"memory-server/adapters/ollama"
	"memory-server/adapters/supabase"
	"memory-server/core"
	"memory-server/internal/config"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP HTTP/SSE server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().Int("port", 7438, "HTTP port")
	serveCmd.Flags().Bool("daemon", false, "Exit silently if already running (for use in hooks)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetInt("port")
	daemon, _ := cmd.Flags().GetBool("daemon")

	// Singleton lock — only one instance per machine
	lockPath := filepath.Join(os.Getenv("USERPROFILE"), ".memory-server", "server.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return fmt.Errorf("cannot create lock dir: %w", err)
	}
	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil || !locked {
		if daemon {
			os.Exit(0)
		}
		return fmt.Errorf("another memory-server instance is already running")
	}
	defer fl.Unlock()

	// Load config
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
		return fmt.Errorf("ERROR: Ollama not reachable at %s — start Ollama before memory-server", cfg.OllamaURL)
	}

	// Supabase store
	store, err := supabase.New(ctx, cfg.SupabaseURL, cfg.SupabaseKey, cfg.DBMaxConns, cfg.DBConnectTimeout)
	if err != nil {
		return fmt.Errorf("ERROR: cannot connect to Supabase: %w", err)
	}

	// Core service
	svc := core.NewMemoryService(ollamaClient, store)

	// Build MCP server
	s := mcpserver.NewMCPServer("memory-server", "1.0.0")

	// ── Tool: remember ────────────────────────────────────────────────────────
	rememberTool := mcp.NewTool("mem_save",
		mcp.WithDescription("Save or upsert a memory observation. Use topic_key to update an existing topic."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Short searchable title (Verb + what)")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Full observation content")),
		mcp.WithString("type", mcp.Required(), mcp.Description("One of: bugfix | decision | architecture | discovery | pattern | config | preference")),
		mcp.WithString("project", mcp.Description("Project slug (inferred from CWD if omitted)")),
		mcp.WithString("scope", mcp.Description("project (default) or personal")),
		mcp.WithString("topic_key", mcp.Description("Stable key for upsert, e.g. architecture/auth-model")),
	)
	s.AddTool(rememberTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// ── Tool: recall ──────────────────────────────────────────────────────────
	recallTool := mcp.NewTool("mem_search",
		mcp.WithDescription("Search memories by semantic similarity."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Natural language search query")),
		mcp.WithString("project", mcp.Description("Limit to a project slug (omit for all projects)")),
		mcp.WithNumber("limit", mcp.Description("Max results (1–50, default 10)")),
	)
	s.AddTool(recallTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// ── Tool: close_session ───────────────────────────────────────────────────
	closeSessionTool := mcp.NewTool("mem_session_summary",
		mcp.WithDescription("Save an end-of-session summary to persistent memory."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project slug")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Full session summary content")),
	)
	s.AddTool(closeSessionTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// ── Tool: get_memory ──────────────────────────────────────────────────────
	getMemoryTool := mcp.NewTool("mem_get_observation",
		mcp.WithDescription("Retrieve a specific memory by its numeric ID."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Numeric memory ID")),
	)
	s.AddTool(getMemoryTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// ── Tool: forget ──────────────────────────────────────────────────────────
	forgetTool := mcp.NewTool("mem_update",
		mcp.WithDescription("Delete the memory matching a project + topic_key pair."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project slug")),
		mcp.WithString("topic_key", mcp.Required(), mcp.Description("Topic key of the memory to delete")),
	)
	s.AddTool(forgetTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Build and start SSE server
	addr := fmt.Sprintf(":%d", port)
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	sseServer := mcpserver.NewSSEServer(s,
		mcpserver.WithBaseURL(baseURL),
		mcpserver.WithStaticBasePath("/mcp"),
	)

	fmt.Printf("memory-server listening on http://localhost:%d/mcp\n", port)

	// Graceful shutdown on SIGINT/SIGTERM
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Println("\nmemory-server shutting down...")
		fl.Unlock()
		os.Exit(0)
	}()

	return sseServer.Start(addr)
}
