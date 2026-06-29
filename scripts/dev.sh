#!/usr/bin/env bash
# Dev mode: Go backend on :8080 + Vite on :5173 (proxying /api).
# Stops on Ctrl+C, killing both processes.
set -euo pipefail
cd "$(dirname "$0")/.."

trap 'kill 0' EXIT INT TERM
(cd backend && go run ./cmd/server) &
(cd frontend && npm run dev) &
wait
