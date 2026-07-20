.PHONY: run dev web test vet migrate tidy neon

export PATH := $(HOME)/.local/go/bin:$(HOME)/go/bin:$(HOME)/.local/bin:$(PATH)

web:
	cd web && npm run build

run: web
	go run ./cmd/schoolmdm

# Vite HMR UI (:5173) + Air Go API (:8080). Open http://127.0.0.1:5173
dev:
	@chmod +x scripts/dev.sh
	@./scripts/dev.sh

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

migrate:
	@echo "Migrations apply automatically on postgres startup when DATABASE_URL is set."

neon:
	@echo "Provisioning claimable Neon Postgres into .env (neon-new)…"
	bash scripts/neon-reclaimable.sh
