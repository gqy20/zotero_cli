.PHONY: build build-server clean release release-server fmt fmt-check lint test vet check install-hooks

BINARY := zot
SERVER := zot-server
EXT := $(shell go env GOEXE)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X zotero_cli/internal/cli.version=$(VERSION) -X zotero_cli/internal/cli.commit=$(COMMIT) -X zotero_cli/internal/cli.buildDate=$(BUILD_DATE) -s -w"
UPX := upx

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
	go build -trimpath $(LDFLAGS) -o $(BINARY)$(EXT) ./cmd/zot

build-server:
	rm -f $(SERVER)$(EXT)
	go build -trimpath $(LDFLAGS) -o $(SERVER)$(EXT) ./cmd/server

# --- 发布（含 upx 压缩）---

release: build
	$(UPX) --best --lzma -o $(BINARY)$(EXT).tmp $(BINARY)$(EXT) && mv $(BINARY)$(EXT).tmp $(BINARY)$(EXT)
	@echo "---"
	@ls -lh $(BINARY)$(EXT)

release-server: build-server
	$(UPX) --best --lzma -o $(SERVER)$(EXT).tmp $(SERVER)$(EXT) && mv $(SERVER)$(EXT).tmp $(SERVER)$(EXT)
	@echo "---"
	@ls -lh $(SERVER)$(EXT)

release-all: release release-server

# --- CI 综合检查 ---

check: fmt-check vet test
	@echo "所有检查通过"

# --- Git Hooks ---

install-hooks:
	@ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
	@echo "pre-commit hook 已安装 → .git/hooks/pre-commit"

# --- 清理 ---

clean:
	rm -f $(BINARY)$(EXT) $(SERVER)$(EXT)
