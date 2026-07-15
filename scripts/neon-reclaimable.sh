#!/usr/bin/env bash
# Create a reclaimable Neon branch and print DATABASE_URL for .env
set -euo pipefail

NAME="${1:-school-mdm-dev}"

if ! command -v neonctl >/dev/null 2>&1; then
  echo "neonctl not found. Install with: npm install -g neonctl" >&2
  exit 1
fi

if [[ -z "${NEON_API_KEY:-}" ]]; then
  echo "Set NEON_API_KEY (Neon console → Account → API keys), then re-run." >&2
  echo "Optional: export NEON_PROJECT_ID=..." >&2
  exit 1
fi

echo "Creating branch: $NAME"
neonctl branches create --name "$NAME" ${NEON_PROJECT_ID:+--project-id "$NEON_PROJECT_ID"} || {
  echo "Branch may already exist; continuing to fetch connection string..." >&2
}

URL="$(neonctl connection-string "$NAME" ${NEON_PROJECT_ID:+--project-id "$NEON_PROJECT_ID"})"
echo
echo "Add to .env:"
echo "DATABASE_URL=$URL"
