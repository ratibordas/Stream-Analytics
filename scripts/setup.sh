#!/usr/bin/env bash
# Install all monorepo dependencies. Run: ./scripts/setup.sh
set -euo pipefail
cd "$(dirname "$0")/.."

err=0
if ! command -v go >/dev/null; then
  echo "✗ Go not found. Install Go >= 1.23: https://go.dev/dl/ (or: brew install go)"
  err=1
fi
if ! command -v node >/dev/null; then
  echo "✗ Node.js not found. Install Node >= 18: https://nodejs.org (or: brew install node)"
  err=1
fi
[ "$err" = 1 ] && exit 1
echo "✓ go: $(go version | awk '{print $3}')  node: $(node --version)"

if [ ! -f .env ]; then
  cp .env.example .env
  echo "✓ created .env (add keys there, or later via the Settings tab in the UI)"
fi

echo "→ go dependencies…"
(cd backend && go mod download)
echo "→ npm dependencies…"
(cd frontend && npm install --no-audit --no-fund)

echo
echo "Done. Dev mode:               ./scripts/dev.sh   (UI at http://localhost:5173)"
echo "Or single production binary:  ./scripts/start.sh (http://localhost:8080)"
