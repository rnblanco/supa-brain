# supa-brain — Claude Code Plugin

Persistent semantic memory for Claude Code, backed by Supabase + pgvector. Gives Claude 9 memory tools (`mem_save`, `mem_search`, `mem_session_summary`, `mem_get_observation`, `mem_update`, `mem_delete`, `mem_context`, `mem_suggest_topic_key`, `mem_capture_passive`) that survive across sessions.

> **Requires Claude Code >= 1.5.0**

---

## Prerequisites

- **supa-brain binary** — download a pre-compiled binary from [GitHub Releases](https://github.com/rnblanco/supa-brain/releases)
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
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.supa-brain"
@'
Set WShell = CreateObject("WScript.Shell")
WShell.Run """C:\Users\<YOUR_USERNAME>\AppData\Local\Programs\Ollama\ollama.exe"" serve", 0, False
'@ | Set-Content "$env:USERPROFILE\.supa-brain\start-ollama.vbs"

# 3. Register in Windows startup (launches silently via wscript.exe — no console window):
Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" `
  -Name "OllamaServe" `
  -Value "wscript.exe $env:USERPROFILE\.supa-brain\start-ollama.vbs"

# 4. Pull the embedding model:
ollama pull nomic-embed-text

# 5. Verify:
curl http://localhost:11434
```

Replace `<YOUR_USERNAME>` with your actual Windows username in the VBS script path.

---

## Step 2 — Install the supa-brain binary

Download the pre-compiled binary for your OS and architecture from [GitHub Releases](https://github.com/rnblanco/supa-brain/releases). Extract the archive and place the binary somewhere in your `PATH` (e.g., `~/.local/bin` on Linux/macOS, `C:\Users\<YOU>\bin` on Windows).

---

## Step 3 — Configure credentials

Create `~/.supa-brain/config.env` with your Supabase connection string:

```bash
mkdir -p ~/.supa-brain
cat > ~/.supa-brain/config.env << 'EOF'
# Supabase Dashboard → Settings → Database → Connection string → URI
# Use the Transaction Pooler (port 6543)
DB_URL=postgresql://postgres.YOUR_REF:YOUR_PASSWORD@aws-0-REGION.pooler.supabase.com:6543/postgres

# Optional — only needed for the db:migrate command
SUPABASE_URL=https://YOUR_REF.supabase.co
EOF
```

Get the `DB_URL` from your Supabase project: **Dashboard → Settings → Database → Connection string → URI mode**. The password is your **Database Password** (not the anon/service key).

---

## Step 4 — Install the plugin

```bash
claude plugin marketplace add rnblanco/supa-brain && claude plugin install supa-brain
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

1. `supa-brain` is in your `PATH` — run `which supa-brain` (Linux/macOS) or `where supa-brain` (Windows)
2. `~/.supa-brain/config.env` exists and contains `DB_URL` with a valid PostgreSQL connection string
3. Ollama is running — `curl http://localhost:11434` should return a response

---

## Requirements

- Claude Code >= 1.5.0
- Supabase project with `pgvector` extension (`CREATE EXTENSION IF NOT EXISTS vector;`)
- Ollama with `nomic-embed-text` model (768-dimensional embeddings)
