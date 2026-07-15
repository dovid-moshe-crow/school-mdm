#!/usr/bin/env bash
# Provision a claimable Neon Postgres DB (no Neon account required).
# Writes DATABASE_URL + PUBLIC_POSTGRES_CLAIM_URL into .env
set -euo pipefail

cd "$(dirname "$0")/.."
ENV_FILE="${1:-.env}"

if grep -q '^DATABASE_URL=postgresql' "$ENV_FILE" 2>/dev/null; then
  echo "DATABASE_URL already set in $ENV_FILE — remove it first to provision a new DB." >&2
  exit 1
fi

echo "Provisioning claimable Neon database via neon-new…"
if command -v npx >/dev/null 2>&1; then
  npx --yes neon-new@latest --yes --env "$ENV_FILE" --key DATABASE_URL --ref school-mdm
else
  echo "npx not found; using neon.new REST API…"
  RESP=$(curl -fsS -X POST "https://neon.new/api/v1/database" \
    -H "Content-Type: application/json" \
    -d '{"ref":"school-mdm"}')
  CONN=$(printf '%s' "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['connection_string'])")
  CLAIM=$(printf '%s' "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['claim_url'])")
  touch "$ENV_FILE"
  grep -v -E '^(DATABASE_URL|PUBLIC_POSTGRES_CLAIM_URL)=' "$ENV_FILE" > "${ENV_FILE}.tmp" || true
  mv "${ENV_FILE}.tmp" "$ENV_FILE"
  printf 'DATABASE_URL=%s\nPUBLIC_POSTGRES_CLAIM_URL=%s\n' "$CONN" "$CLAIM" >> "$ENV_FILE"
fi

grep -q '^HTTP_ADDR=' "$ENV_FILE" || echo 'HTTP_ADDR=:8080' >> "$ENV_FILE"
grep -q '^ADMIN_TOKENS=' "$ENV_FILE" || echo 'ADMIN_TOKENS=dev-admin-token' >> "$ENV_FILE"
grep -q '^PORTAL_BASE_URL=' "$ENV_FILE" || echo 'PORTAL_BASE_URL=http://localhost:8080' >> "$ENV_FILE"

echo
echo "Done. Redacted:"
grep -E '^(DATABASE_URL|PUBLIC_POSTGRES_CLAIM_URL)=' "$ENV_FILE" | sed -E 's#(postgresql://[^:]+:)[^@]+@#\1***@#'
echo
echo "Claim within 72h to keep the DB: see PUBLIC_POSTGRES_CLAIM_URL in $ENV_FILE"
echo "Then: make run   (or make dev)"
