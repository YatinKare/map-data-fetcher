#!/usr/bin/env bash
set -euo pipefail

EXTERNAL_PATH="${TAILSCALE_MCP_PATH:-/naver/mcp}"
LOCAL_MCP_URL="${LOCAL_MCP_URL:-http://127.0.0.1:3000/mcp}"
MODE="${TAILSCALE_MODE:-serve}"
HTTPS_PORT="${TAILSCALE_HTTPS_PORT:-10000}"

case "$MODE" in
  serve)
    tailscale serve --https="$HTTPS_PORT" --bg --set-path="$EXTERNAL_PATH" "$LOCAL_MCP_URL"
    ;;
  funnel)
    tailscale funnel --https="$HTTPS_PORT" --bg --set-path="$EXTERNAL_PATH" "$LOCAL_MCP_URL"
    ;;
  *)
    printf 'TAILSCALE_MODE must be serve or funnel, got: %s\n' "$MODE" >&2
    exit 2
    ;;
esac

printf 'Configured Tailscale %s route on HTTPS port %s: %s -> %s\n' \
  "$MODE" "$HTTPS_PORT" "$EXTERNAL_PATH" "$LOCAL_MCP_URL"
tailscale serve status
