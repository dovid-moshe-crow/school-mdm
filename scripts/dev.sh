#!/usr/bin/env bash
# Start Air (Go API) first, wait until it listens, then start Vite (UI HMR).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="${HOME}/.local/go/bin:${HOME}/go/bin:${HOME}/.local/bin:${PATH}"

if ! command -v air >/dev/null 2>&1; then
  echo "Installing air…"
  go install github.com/air-verse/air@latest
fi

if [[ ! -d web/node_modules ]]; then
  echo "Installing web dependencies…"
  (cd web && npm install)
fi

# Go still embeds dist at compile time even when the browser uses Vite.
if [[ ! -f internal/webui/dist/index.html ]]; then
  echo "Building UI once for Go embed…"
  (cd web && npm run build)
fi

mkdir -p tmp

air_pid=""
vite_pid=""
cleanup() {
  if [[ -n "${vite_pid}" ]] && kill -0 "${vite_pid}" 2>/dev/null; then
    kill "${vite_pid}" 2>/dev/null || true
    wait "${vite_pid}" 2>/dev/null || true
  fi
  if [[ -n "${air_pid}" ]] && kill -0 "${air_pid}" 2>/dev/null; then
    kill "${air_pid}" 2>/dev/null || true
    wait "${air_pid}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

strip_proxy=(env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy -u ALL_PROXY -u all_proxy)

echo "Starting API (Air) on http://127.0.0.1:8080 …"
"${strip_proxy[@]}" air -c .air.toml &
air_pid=$!

echo "Waiting for API to accept connections…"
ready=0
for _ in $(seq 1 90); do
  if ! kill -0 "${air_pid}" 2>/dev/null; then
    echo "Air exited before the API became ready." >&2
    exit 1
  fi
  if curl -sf --max-time 1 "http://127.0.0.1:8080/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.4
done
if [[ "${ready}" -ne 1 ]]; then
  echo "Timed out waiting for http://127.0.0.1:8080/healthz" >&2
  exit 1
fi

echo "API ready."
echo "UI (Vite HMR):  http://127.0.0.1:5173"
echo "Open the Vite URL for React Grab / hot reload."
echo

(
  cd web
  "${strip_proxy[@]}" npm run dev -- --host 127.0.0.1 --port 5173 --strictPort
) &
vite_pid=$!

# Keep the script alive while either child runs; prefer Air as the "main" process.
wait "${air_pid}"
