# supa-brain

Persistent semantic memory MCP server for Claude Code — powered by Supabase + pgvector + Ollama.

Replaces keyword search with real semantic similarity so you can query with natural language across sessions.

## Install as Claude Code plugin

```bash
claude plugin marketplace add rnblanco/supa-brain
claude plugin install supa-brain
```

Full setup guide (Ollama, Supabase, credentials): [plugin/claude-code/README.md](plugin/claude-code/README.md)

## Install binary manually

```bash
go install github.com/rnblanco/supa-brain@latest
```

Or download a pre-built binary from [GitHub Releases](https://github.com/rnblanco/supa-brain/releases).

## MCP Tools

| Tool | Description |
|------|-------------|
| `mem_save` | Save an observation with semantic embedding |
| `mem_search` | Search by natural language query |
| `mem_session_summary` | Summarize and persist a session |
| `mem_get_observation` | Retrieve observation by ID |
| `mem_update` | Patch an existing observation by ID |
| `mem_delete` | Delete observation by project + topic_key |
| `mem_context` | Get recent sessions and observations (no embedding cost) |
| `mem_suggest_topic_key` | Generate a stable slug for upserts |
| `mem_capture_passive` | Extract learnings from Key Learnings sections |

## Configuration

Create `~/.supa-brain/config.env`:

```env
SUPABASE_URL=https://xxxxx.supabase.co
SUPABASE_KEY=eyJ...
```

## Requirements

- Ollama running locally with `nomic-embed-text` model
- Supabase project with pgvector extension enabled
- Claude Code >= 1.5.0
