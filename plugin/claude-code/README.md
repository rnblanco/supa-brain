# memory-server — Claude Code Plugin

Persistent semantic memory for Claude Code, backed by Supabase + pgvector. Gives Claude 9 memory tools (`mem_save`, `mem_search`, `mem_session_summary`, `mem_get_observation`, `mem_update`, `mem_delete`, `mem_context`, `mem_suggest_topic_key`, `mem_capture_passive`) that survive across sessions.

> **Requires Claude Code >= 1.5.0**

---

## Prerequisites

- **Go 1.24+** (to install via `go install`) — OR download a pre-compiled binary from [GitHub Releases](https://github.com/Gentleman-Programming/memory-server/releases)
- **Ollama** running locally with the `nomic-embed-text` model pulled
- **Supabase** project with the `pgvector` extension enabled

---

## Step 1 — Install Ollama

### Linux (systemd — automatic)

```bash
curl -fsSL https://ollama.com/install.sh | sh
# The installer creates the systemd service automatically
systemctl --user status ollama   # verify it is running
ollama pull nomic-embed-text
curl http://localhost:11434      # should return OK
```

### macOS

```bash
brew install ollama
brew services start ollama       # starts as a background service
ollama pull nomic-embed-text
curl http://localhost:11434      # should return OK
```

### Windows (silent startup — no console window)

```powershell
# 1. Download and install Ollama from https://ollama.com/download/windows

# 2. Create the VBS wrapper for silent startup:
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.memory-server"
@'
Set WShell = CreateObject("WScript.Shell")
WShell.Run """C:\Users\<YOUR_USERNAME>\AppData\Local\Programs\Ollama\ollama.exe"" serve", 0, False
'@ | Set-Content "$env:USERPROFILE\.memory-server\start-ollama.vbs"

# 3. Register in Windows startup (launches silently via wscript.exe — no console window):
Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" `
  -Name "OllamaServe" `
  -Value "wscript.exe $env:USERPROFILE\.memory-server\start-ollama.vbs"

# 4. Pull the embedding model:
ollama pull nomic-embed-text

# 5. Verify:
curl http://localhost:11434
```

Replace `<YOUR_USERNAME>` with your actual Windows username in the VBS script path.

---

## Step 2 — Install the memory-server binary

**Option A — Go users:**

```bash
go install github.com/Gentleman-Programming/memory-server@latest
```

**Option B — Non-Go users:**

Download the pre-compiled binary for your OS and architecture from [GitHub Releases](https://github.com/Gentleman-Programming/memory-server/releases). Extract the archive and place the binary somewhere in your `PATH` (e.g., `~/.local/bin` on Linux/macOS, `C:\Users\<YOU>\bin` on Windows).

---

## Step 3 — Configure credentials

Create `~/.memory-server/config.env` with your Supabase credentials:

```bash
mkdir -p ~/.memory-server
cat > ~/.memory-server/config.env << 'EOF'
SUPABASE_URL=https://xxxxx.supabase.co
SUPABASE_KEY=eyJ...
EOF
```

The `SUPABASE_KEY` can be the anon key or a service role key from your Supabase project settings.

---

## Step 4 — Install the plugin

```bash
claude plugin marketplace add Gentleman-Programming/memory-server
claude plugin install memory-server
```

Claude Code will register the MCP server and the `SessionStart` hook automatically.

---

## Verification

Open a new Claude Code session and run:

```
mem_save
```

If Claude responds with a tool call acknowledgment, all 9 memory tools are available and the server is running correctly.

You can also check that the following tools are listed in your session:
`mem_save`, `mem_search`, `mem_session_summary`, `mem_get_observation`, `mem_update`, `mem_delete`, `mem_context`, `mem_suggest_topic_key`, `mem_capture_passive`

---

## Troubleshooting

If the `SessionStart` hook reports missing prerequisites, check:

1. `memory-server` is in your `PATH` — run `which memory-server` (Linux/macOS) or `where memory-server` (Windows)
2. `~/.memory-server/config.env` exists and contains `SUPABASE_URL` and `SUPABASE_KEY`
3. Ollama is running — `curl http://localhost:11434` should return a response

---

## Requirements

- Claude Code >= 1.5.0
- Supabase project with `pgvector` extension (`CREATE EXTENSION IF NOT EXISTS vector;`)
- Ollama with `nomic-embed-text` model (768-dimensional embeddings)
