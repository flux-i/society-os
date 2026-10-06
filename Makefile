.PHONY: setup build check seed run dev snapshot restore-check bench report browser-check webmcp-check eval

DB ?= var/demo/society.db
ADDR ?= 127.0.0.1:8080
SNAPSHOT ?= var/snapshots/local-$(shell date +%Y%m%d-%H%M%S)
RESTORED_DB ?= var/restored/check-$(shell date +%Y%m%d-%H%M%S)/society.db
MFA_KEY_FILE ?= var/keys/mfa.key
MESSAGE_KEY_FILE ?= var/keys/messages.key
REPORT ?= reports/local/account-security-baseline.json
# Full race coverage retains real password hashing and isolated fixtures.
GO_TEST_TIMEOUT ?= 20m

setup:
	go mod download
	npm ci --prefix web

build:
	npm run build --prefix web
	go build -trimpath -o build/society-server ./cmd/society-server

check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)
	go vet ./...
	go test -race -timeout "$(GO_TEST_TIMEOUT)" ./...
	npm run check --prefix web

seed: build
	./build/society-server seed-demo --demo --db "$(DB)"

run: seed
	./build/society-server serve --demo --db "$(DB)" --mfa-key-file "$(MFA_KEY_FILE)" --message-key-file "$(MESSAGE_KEY_FILE)" --addr "$(ADDR)" --web-dir build/web

dev:
	npm run dev --prefix web

snapshot: build
	./build/society-server snapshot --db "$(DB)" --out "$(SNAPSHOT)"

restore-check: build
	./build/society-server restore-check --snapshot "$(SNAPSHOT)" --out "$(RESTORED_DB)"

bench:
	go test ./internal/database -run '^$$' -bench BenchmarkRegistryRead -benchmem -count=3

report: build
	python3 scripts/baseline.py --binary build/society-server --web-dir build/web --out "$(REPORT)"

browser-check: build
	npm run test:browser --prefix web

webmcp-check: build
	npm run test:browser --prefix web -- --config=playwright.webmcp.config.ts

# One checkpoint gate; every stage must finish successfully before proceeding.
eval: check build
	npm run test:browser --prefix web
	npm run test:browser --prefix web -- --config=playwright.webmcp.config.ts
