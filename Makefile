# Space — Ruang Kerja HIMASIF. Run from the repository root.
# Development configuration lives outside the repository (see .env.example).

ENV_FILE ?= $(HOME)/.config/himasif-space/dev.env
GO       ?= go
WEB_OUT  := app/build/web

.PHONY: all check fmt vet test test-db web desktop-windows server serve migrate seed mail-test clean

all: check

check: fmt vet test

fmt:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files above need formatting"; exit 1; }

vet:
	$(GO) vet ./...
	GOOS=windows $(GO) vet ./app ./internal/...
	GOOS=js GOARCH=wasm $(GO) vet ./app ./internal/appui ./internal/client ./internal/api

# Unit tests; the server's database tests skip without SPACE_TEST_DATABASE_URL.
test:
	$(GO) test ./...

# All tests, with the database tests against SPACE_TEST_DATABASE_URL of ENV_FILE.
test-db:
	SPACE_TEST_DATABASE_URL="$$(grep '^SPACE_TEST_DATABASE_URL=' $(ENV_FILE) | cut -d= -f2-)" $(GO) test -count=1 ./...

# The web app (native UI compiled to WebAssembly), with a gzipped copy of
# the WebAssembly for the server to send.
web:
	$(GO) tool mygo build -platform web ./app
	gzip -kf9 $(WEB_OUT)/*.wasm

desktop-windows:
	$(GO) tool mygo build -platform windows/amd64 ./app

server:
	$(GO) build -o bin/space-server ./cmd/space-server

serve: server
	./bin/space-server -env-file $(ENV_FILE) serve

migrate: server
	./bin/space-server -env-file $(ENV_FILE) migrate

seed: server
	./bin/space-server -env-file $(ENV_FILE) seed

mail-test: server
	./bin/space-server -env-file $(ENV_FILE) mail-test -to $(TO)

clean:
	rm -rf bin app/build
