.PHONY: build clean release fmt fmt-check lint test vet check install-hooks

BINARY := zot
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X zotero_cli/internal/cli.version=$(VERSION) -X zotero_cli/internal/cli.commit=$(COMMIT) -X zotero_cli/internal/cli.buildDate=$(BUILD_DATE) -s -w"
UPX := tools/upx.exe
UPX_VERSION := 4.2.4
UPX_URL := https://github.com/upx/upx/releases/download/v$(UPX_VERSION)/upx-$(UPX_VERSION)-win64.zip

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
	rm -f $(BINARY).exe $(BINARY)-compressed.exe
	go build -trimpath $(LDFLAGS) -o $(BINARY).exe ./cmd/zot

# --- 发布（含 upx 压缩）---

$(UPX):
	@mkdir -p tools
	curl -sL "$(UPX_URL)" -o /tmp/upx.zip
	unzip -o /tmp/upx.zip -d /tmp/upx_pkg
	cp /tmp/upx_pkg/upx-$(UPX_VERSION)-win64/upx.exe $(UPX)
	rm -rf /tmp/upx.zip /tmp/upx_pkg

release: build | $(UPX)
	$(UPX) --best --lzma -o $(BINARY).exe.tmp $(BINARY).exe && mv $(BINARY).exe.tmp $(BINARY).exe
	@echo "---"
	@ls -lh $(BINARY).exe

# --- CI 综合检查 ---

check: fmt-check vet test
	@echo "所有检查通过"

# --- Git Hooks ---

install-hooks:
	@ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
	@echo "pre-commit hook 已安装 → .git/hooks/pre-commit"

# --- 清理 ---

clean:
	rm -f $(BINARY).exe
