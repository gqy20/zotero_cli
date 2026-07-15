package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/config"
)

const defaultTasteTemplate = `---
version: 1
scope: zotero-library-management
---

# Zotero Library Taste

## 核心原则

- 在这里记录稳定的文献管理偏好，不记录逐批操作日志。
- 当前用户请求优先于本文件；本文件优先于工具默认行为。

## 受保护标签

- 在这里列出不应自动修改的状态标签。

## 一级方向

- 在这里列出需要作为一级标签保留的核心研究方向。

## 标签决策规则

- 在这里记录层级深度、同义词、总类与细分类并存等规则。

## 典型案例

- 在这里记录少量能够说明边界的正例和反例。

## 待确认事项

- 尚未形成稳定偏好的问题放在这里，不要提前固化。
`

type LibraryTaste struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content,omitempty"`
}

func ResolveLibraryTastePath(cfg config.Config, configPath string) string {
	if dataDir := strings.TrimSpace(cfg.DataDir); dataDir != "" {
		return filepath.Join(dataDir, ".zotero_cli", "taste.md")
	}
	if strings.TrimSpace(configPath) != "" {
		return filepath.Join(filepath.Dir(configPath), ".zotero_cli", "taste.md")
	}
	return ""
}

func LoadLibraryTaste(cfg config.Config, configPath string) (LibraryTaste, error) {
	path := ResolveLibraryTastePath(cfg, configPath)
	taste := LibraryTaste{Path: path}
	if path == "" {
		return taste, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return taste, nil
	}
	if err != nil {
		return LibraryTaste{}, fmt.Errorf("read library taste: %w", err)
	}
	taste.Exists = true
	taste.Content = string(data)
	return taste, nil
}

func (s ReadService) Taste(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cfg, configPath, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	taste, err := LoadLibraryTaste(cfg, configPath)
	if err != nil {
		return Result{}, err
	}
	meta := map[string]any{"taste_path": taste.Path, "taste_exists": taste.Exists}
	if !taste.Exists {
		return Result{
			Data:     taste,
			Meta:     meta,
			Text:     fmt.Sprintf("library taste is not configured\nCreate: zot lib taste --init\nPath: %s", taste.Path),
			Warnings: []Warning{{Code: "taste_missing", Message: "library taste is not configured; run `zot lib taste --init`"}},
		}, nil
	}
	return Result{Data: taste, Meta: meta, Text: taste.Content}, nil
}

func (s ReadService) TastePath(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cfg, configPath, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	taste, err := LoadLibraryTaste(cfg, configPath)
	if err != nil {
		return Result{}, err
	}
	status := LibraryTaste{Path: taste.Path, Exists: taste.Exists}
	return Result{Data: status, Meta: map[string]any{"taste_exists": taste.Exists}, Text: taste.Path}, nil
}

func (s ReadService) InitTaste(ctx context.Context, force bool) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cfg, configPath, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	path := ResolveLibraryTastePath(cfg, configPath)
	if path == "" {
		return Result{}, fmt.Errorf("cannot resolve library taste path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, fmt.Errorf("create library taste directory: %w", err)
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if os.IsExist(err) {
		return Result{}, fmt.Errorf("library taste already exists at %s; use --force to overwrite", path)
	}
	if err != nil {
		return Result{}, fmt.Errorf("create library taste: %w", err)
	}
	if _, err := file.WriteString(defaultTasteTemplate); err != nil {
		_ = file.Close()
		return Result{}, fmt.Errorf("write library taste: %w", err)
	}
	if err := file.Close(); err != nil {
		return Result{}, fmt.Errorf("close library taste: %w", err)
	}
	taste := LibraryTaste{Path: path, Exists: true, Content: defaultTasteTemplate}
	return Result{Data: taste, Meta: map[string]any{"taste_path": path, "taste_exists": true}, Text: "created library taste: " + path}, nil
}
