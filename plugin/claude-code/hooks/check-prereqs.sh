#!/usr/bin/env bash
ERRORS=()
if ! command -v memory-server &>/dev/null; then
  ERRORS+=("memory-server not found in PATH. Install: go install github.com/Gentleman-Programming/memory-server@latest")
fi
CONFIG="${HOME}/.memory-server/config.env"
if [[ -f "$CONFIG" ]]; then source "$CONFIG" 2>/dev/null; fi
for VAR in SUPABASE_URL SUPABASE_KEY; do
  if [[ -z "${!VAR:-}" ]]; then
    ERRORS+=("$VAR not set. Add it to ~/.memory-server/config.env")
  fi
done
if ! curl -sf http://localhost:11434 &>/dev/null; then
  ERRORS+=("Ollama is not running. Start it with: ollama serve (or configure it as a service — see plugin README)")
fi
if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "memory-server plugin: prerequisites missing:" >&2
  for ERR in "${ERRORS[@]}"; do echo "  - $ERR" >&2; done
fi
exit 0
