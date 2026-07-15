.PHONY: run dev test vet migrate tidy neon

export PATH := $(HOME)/.local/go/bin:$(HOME)/go/bin:$(HOME)/.local/bin:$(PATH)

run:
	go run ./cmd/schoolmdm

# Live reload: rebuild + restart on Go/HTML/SQL changes.
# Usage: make dev
dev:
	@command -v air >/dev/null 2>&1 || { \
	  echo "Installing air…"; \
	  go install github.com/air-verse/air@latest; \
	}
	@mkdir -p tmp
	env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy -u ALL_PROXY -u all_proxy \
	  air -c .air.toml

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
