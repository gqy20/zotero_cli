.PHONY: build clean release fmt fmt-check lint test vet check bench-cli bench-cli-data install-hooks

BINARY := zot
EXT := $(shell go env GOEXE)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X zotero_cli/internal/cli.version=$(VERSION) -X zotero_cli/internal/cli.commit=$(COMMIT) -X zotero_cli/internal/cli.buildDate=$(BUILD_DATE) -s -w"
UPX := upx
DIST := dist

# --- 格式化 ---

fmt:
	gofmt -w ./internal/ ./cmd/

fmt-check:
	@unformatted="$$(gofmt -l ./internal/ ./cmd/)"; \
	if [ -n "$${unformatted}" ]; then \
		echo "以下文件需要 gofmt 格式化:"; \
		echo "$${unformatted}"; \
		exit 1; \
	fi; \
	echo "格式检查通过"

# --- 静态分析 ---

vet:
	go vet ./...

lint: vet fmt-check

# --- 测试 ---

test:
	go test ./... -v

test-short:
	go test ./... -short

# --- 构建 ---

build:
	rm -f $(BINARY)$(EXT)
	@if [ "$(shell go env GOOS)" != "darwin" ]; then command -v $(UPX) >/dev/null 2>&1 || { echo "UPX is required: install upx and retry" >&2; exit 1; }; fi
	go build -trimpath $(LDFLAGS) -o $(BINARY)$(EXT) ./cmd/zot
	@if [ "$(shell go env GOOS)" = "darwin" ]; then \
		echo "macOS binary left uncompressed (UPX is not used for Mach-O)"; \
	else \
		$(UPX) --best --lzma -o $(BINARY)$(EXT).tmp $(BINARY)$(EXT) && \
		mv $(BINARY)$(EXT).tmp $(BINARY)$(EXT) && \
		$(UPX) -t $(BINARY)$(EXT); \
	fi
	@ls -lh $(BINARY)$(EXT)

# --- 发布（含 upx 压缩）---

release:
	go run ./scripts/release.go \
		-version "$(VERSION)" \
		-commit "$(COMMIT)" \
		-build-date "$(BUILD_DATE)" \
		-dist "$(DIST)" \
		-upx "$(UPX)"

# --- CI 综合检查 ---

check: fmt-check vet test
	@echo "所有检查通过"

# --- Git Hooks ---

bench-cli: build
	go run ./cmd/zot-bench --binary ./zot$(EXT) --mode all --tier default

bench-cli-data: build
	go run ./cmd/zot-bench --binary ./zot$(EXT) --mode all --tier data $(ARGS)

install-hooks:
	@ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
	@ln -sf ../../scripts/commit-msg .git/hooks/commit-msg
	@echo "commit-msg hook installed -> .git/hooks/commit-msg"
	@echo "pre-commit hook 已安装 → .git/hooks/pre-commit"

# --- 清理 ---

clean:
	rm -f $(BINARY)$(EXT)
	rm -rf $(DIST)
