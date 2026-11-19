SHELL := /bin/sh
GO ?= go
export GOTOOLCHAIN := auto
export GOCACHE := $(CURDIR)/.cache/build
export GOMODCACHE := $(CURDIR)/.cache/mod
PREFIX ?= $(HOME)/.local
DESTDIR ?=
TARGET ?=
VERSION ?= dev
TEST_RUN ?= .
LDFLAGS := -s -w -X github.com/UcGeorge-Canonical/easy-lima/internal/model.Version=$(VERSION)

.PHONY: build fmt fmt-check vet test test-race test-contracts test-acceptance check clean
build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/easy-lima ./cmd/easy-lima
fmt:
	gofmt -w cmd internal assets testsupport scripts
fmt-check:
	@test -z "$$(gofmt -l cmd internal assets testsupport scripts)" || (gofmt -l cmd internal assets testsupport scripts; exit 1)
vet:
	$(GO) vet ./...
test:
	$(GO) test ./...
test-race:
	$(GO) test -race ./...
test-contracts:
	$(GO) test -tags contracts ./...
	python3 -m unittest discover -s testsupport/guest -p 'test_*.py'
test-acceptance:
	$(GO) test -v -tags acceptance -timeout 45m -run '$(TEST_RUN)' ./testsupport/acceptance
check: fmt-check vet test docs-check build
clean:
	rm -rf bin dist coverage.out

.PHONY: install uninstall licenses release tidy build-cross
install: build
	install -d "$(DESTDIR)$(PREFIX)/bin" "$(DESTDIR)$(PREFIX)/share/doc/easy-lima" "$(DESTDIR)$(PREFIX)/share/bash-completion/completions" "$(DESTDIR)$(PREFIX)/share/zsh/site-functions" "$(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d"
	install -m 755 bin/easy-lima "$(DESTDIR)$(PREFIX)/bin/easy-lima"
	install -m 644 README.md LICENSE NOTICE THIRD_PARTY_NOTICES.txt "$(DESTDIR)$(PREFIX)/share/doc/easy-lima/"
	cp -R docs "$(DESTDIR)$(PREFIX)/share/doc/easy-lima/"
	install -d "$(DESTDIR)$(PREFIX)/share/doc/easy-lima/.github"
	install -m 644 .github/CONTRIBUTING.md .github/SECURITY.md "$(DESTDIR)$(PREFIX)/share/doc/easy-lima/.github/"
	install -m 644 packaging/completions/easy-lima.bash "$(DESTDIR)$(PREFIX)/share/bash-completion/completions/easy-lima"
	install -m 644 packaging/completions/_easy-lima "$(DESTDIR)$(PREFIX)/share/zsh/site-functions/_easy-lima"
	install -m 644 packaging/completions/easy-lima.fish "$(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/easy-lima.fish"
uninstall:
	rm -f "$(DESTDIR)$(PREFIX)/bin/easy-lima"
	rm -f "$(DESTDIR)$(PREFIX)/share/bash-completion/completions/easy-lima" "$(DESTDIR)$(PREFIX)/share/zsh/site-functions/_easy-lima" "$(DESTDIR)$(PREFIX)/share/fish/vendor_completions.d/easy-lima.fish"
	rm -rf "$(DESTDIR)$(PREFIX)/share/doc/easy-lima"
licenses:
	$(GO) mod download
	python3 scripts/licenses.py
release:
	python3 scripts/release.py --version '$(VERSION)' --target '$(TARGET)'
tidy:
	$(GO) mod tidy
build-cross:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -o bin/easy-lima-linux-arm64 ./cmd/easy-lima
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o bin/easy-lima-linux-amd64 ./cmd/easy-lima

.PHONY: build-linux-tests
build-linux-tests:
	@mkdir -p bin/linux-tests
	@for package in host/agent state/sqlite config app; do CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) test -c -tags contracts -o "bin/linux-tests/$$(basename $$package).test" "./internal/$$package" || exit; done

.PHONY: docs-check
docs-check:
	python3 scripts/check_docs.py
	bash -n packaging/completions/easy-lima.bash
	@if command -v zsh >/dev/null 2>&1; then zsh -n packaging/completions/_easy-lima; fi
	@if command -v fish >/dev/null 2>&1; then fish -n packaging/completions/easy-lima.fish; fi

.PHONY: docs-generate
docs-generate:
	$(GO) run ./scripts/docgen

.PHONY: test-terminal
test-terminal: build
	python3 -m unittest discover -s testsupport/terminal -p 'test_*.py'
