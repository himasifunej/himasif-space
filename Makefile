# Space — Ruang Kerja HIMASIF. Run from the repository root.
# Development configuration lives outside the repository (see .env.example).

ENV_FILE ?= $(HOME)/.config/himasif-space/dev.env
GO       ?= go
# Python 3. Windows installs it as `python` (or the `py` launcher), and its
# `python3` is often only the Microsoft Store stub; override with
# `make PYTHON=py` if needed. Make on Windows needs a POSIX shell (Git Bash).
ifeq ($(OS),Windows_NT)
PYTHON   ?= python
else
PYTHON   ?= python3
endif
WEB_OUT  := app/build/web
VERSION  := $(shell $(PYTHON) -c "import json;print(json.load(open('app/mygo.json'))['version'])")
# Shown on Status layanan: the commit (with -dirty for local changes), a
# SHA-256 over the sorted per-file SHA-256 of the tracked files as they are
# on disk, and the build time.
REVISION := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)$(shell git diff --quiet HEAD 2>/dev/null || echo -dirty)
DIGEST   := $(shell git ls-files -z | xargs -0 sha256sum 2>/dev/null | LC_ALL=C sort -k2 | sha256sum | cut -c1-64)
BUILT_AT := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
SERVER_PKG := github.com/himasifunej/himasif-space/internal/server
SERVER_LDFLAGS := -X $(SERVER_PKG).Version=$(VERSION) -X $(SERVER_PKG).SourceRevision=$(REVISION) -X $(SERVER_PKG).SourceDigest=$(DIGEST) -X $(SERVER_PKG).BuiltAt=$(BUILT_AT)

.PHONY: all check fmt vet test test-ops test-db web desktop-windows server serve migrate seed mail-test clean

all: check

check: fmt vet test test-ops

fmt:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files above need formatting"; exit 1; }

vet:
	$(GO) vet ./...
	GOOS=windows $(GO) vet ./app ./internal/...
	GOOS=js GOARCH=wasm $(GO) vet ./app ./internal/appui ./internal/client ./internal/api

# Unit tests; the server's database tests skip without SPACE_TEST_DATABASE_URL.
test:
	$(GO) test ./...

# The ops scripts' unit tests; the ones that need the Linux server
# (POSIX modes, systemd) skip on Windows, test_restore_integration needs a DB.
test-ops:
	cd scripts/ops && $(PYTHON) -m unittest test_ops

# All tests, with the database tests against SPACE_TEST_DATABASE_URL of ENV_FILE.
test-db:
	SPACE_TEST_DATABASE_URL="$$(grep '^SPACE_TEST_DATABASE_URL=' $(ENV_FILE) | cut -d= -f2-)" $(GO) test -count=1 ./...

# The web app (native UI compiled to WebAssembly), with a gzipped copy of
# the WebAssembly for the server to send.
web:
	MYGO_LDFLAGS="-X main.version=$(VERSION)" $(GO) tool mygo build -platform web ./app
	$(PYTHON) scripts/release/patch_web_index.py $(WEB_OUT)/index.html
	gzip -kf9 $(WEB_OUT)/*.wasm

desktop-windows:
	$(GO) tool mygo build -platform windows/amd64 ./app

server:
	$(GO) build -ldflags "$(SERVER_LDFLAGS)" -o bin/space-server ./cmd/space-server

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
