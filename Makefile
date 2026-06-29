.PHONY: dev backend frontend build run

# dev: backend on :8080 + vite on :5173 (vite proxies /api)
dev:
	(cd backend && go run ./cmd/server) & (cd frontend && npm run dev) & wait

backend:
	cd backend && go run ./cmd/server

frontend:
	cd frontend && npm run dev

# production: build the frontend and serve it from the Go binary on :8080
build:
	cd frontend && npm install && npm run build
	cd backend && go build -o ../bin/server ./cmd/server

run: build
	cd backend && ../bin/server
