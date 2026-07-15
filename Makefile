.PHONY: run test vet migrate tidy neon

export PATH := $(HOME)/.local/go/bin:$(PATH)

run:
	go run ./cmd/schoolmdm

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

migrate:
	@echo "Migrations apply automatically on postgres startup when DATABASE_URL is set."

neon:
	bash scripts/neon-reclaimable.sh school-mdm-dev
