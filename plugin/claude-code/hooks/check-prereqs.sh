#!/usr/bin/env bash
ERRORS=()
if ! command -v supa-brain &>/dev/null; then
  ERRORS+=("supa-brain not found in PATH. Download a binary from: https://github.com/rnblanco/supa-brain/releases")
fi
CONFIG="${HOME}/.supa-brain/config.env"
if [[ -f "$CONFIG" ]]; then source "$CONFIG" 2>/dev/null; fi
for VAR in SUPABASE_URL SUPABASE_KEY; do
  if [[ -z "${!VAR:-}" ]]; then
    ERRORS+=("$VAR not set. Add it to ~/.supa-brain/config.env")
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
