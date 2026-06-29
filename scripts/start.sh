#!/usr/bin/env bash
# Production mode: build frontend + backend and run everything as one binary
# at http://localhost:8080 (the Go server serves the built frontend itself).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "→ building frontend…"
(cd frontend && npm run build)
echo "→ building backend…"
mkdir -p bin
(cd backend && go build -o ../bin/server ./cmd/server)
echo "→ starting: http://localhost:8080"
cd backend && exec ../bin/server
