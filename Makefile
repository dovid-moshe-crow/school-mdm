.PHONY: run build-bin dev web test vet migrate tidy neon mdmimport

export PATH := $(HOME)/.local/go/bin:$(HOME)/go/bin:$(HOME)/.local/bin:$(PATH)

# Fixed output path so Windows Firewall remembers a single allowed binary
# (go run writes a new temp .exe every launch and re-prompts).
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/schoolmdm$(if $(filter Windows_NT,$(OS)),.exe,)

web:
	cd web && npm run build

build-bin:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) ./cmd/schoolmdm

run: web build-bin
	$(BIN)

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

mdmimport:
	go run ./cmd/mdmimport -dry-run
