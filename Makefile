.PHONY: run dev web test vet migrate tidy neon

export PATH := $(HOME)/.local/go/bin:$(HOME)/go/bin:$(HOME)/.local/bin:$(PATH)

web:
	cd web && npm run build

run: web
	go run ./cmd/schoolmdm

# Live reload (Go). Rebuild UI with `make web` or `cd web && npm run dev` (proxy to :8080).
dev:
	@command -v air >/dev/null 2>&1 || { \
	  echo "Installing air…"; \
	  go install github.com/air-verse/air@latest; \
	}
	@mkdir -p tmp
	@test -f internal/webui/dist/index.html || $(MAKE) web
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
