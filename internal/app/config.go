package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
	"zotero_cli/internal/zoteroapi"
)

type ConfigService struct{}

type ConfigInitRequest struct {
	Mode        string
	LibraryType string
	LibraryID   string
	APIKey      string
	DataDir     string
	ServerAddr  string
	SetupPDF    bool
	NoPDF       bool
	CheckPDF    bool
	Prompt      func(config.Config, map[string]bool) (config.Config, error)
	ConfirmPDF  func() (bool, error)
}

func (ConfigService) Init(ctx context.Context, req ConfigInitRequest) (Result, error) {
	if req.CheckPDF {
		cfg, _, err := config.Load()
		if err != nil {
			return Result{}, err
		}
		return checkPDFResult(cfg)
	}
	path, err := config.DefaultPath()
	if err != nil {
		return Result{}, err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		if !req.SetupPDF {
			return Result{}, fmt.Errorf("config already exists at %s; edit it manually, or remove it before re-running init", path)
		}
		cfg, _, loadErr := config.Load()
		if loadErr != nil {
			return Result{}, loadErr
		}
		return setupPDFResult(ctx, cfg)
	} else if !os.IsNotExist(statErr) {
		return Result{}, statErr
	}

	cfg := config.Default()
	if req.Mode != "" {
		cfg.Mode = req.Mode
	}
	if req.LibraryType != "" {
		cfg.LibraryType = req.LibraryType
	}
	if req.LibraryID != "" {
		cfg.LibraryID = req.LibraryID
	}
	if req.APIKey != "" {
		cfg.APIKey = req.APIKey
	}
	if req.DataDir != "" {
		cfg.DataDir = req.DataDir
	}
	if req.ServerAddr != "" {
		cfg.ServerAddr = req.ServerAddr
	}
	provided := map[string]bool{
		"mode": req.Mode != "", "library_type": req.LibraryType != "", "library_id": req.LibraryID != "",
		"api_key": req.APIKey != "", "data_dir": req.DataDir != "", "server_addr": req.ServerAddr != "",
	}
	nonInteractive := (provided["mode"] && provided["library_type"] && provided["library_id"] && provided["api_key"]) ||
		(cfg.Mode == "remote" && provided["server_addr"] && (!provided["api_key"] || (provided["library_id"] && provided["api_key"])))
	if !(nonInteractive && (cfg.Mode == "web" || cfg.Mode == "remote" || provided["data_dir"])) {
		if req.Prompt == nil {
			return Result{}, fmt.Errorf("interactive configuration requires a prompt adapter")
		}
		cfg, err = req.Prompt(cfg, provided)
		if err != nil {
			return Result{}, err
		}
	}
	if err := config.Save(cfg); err != nil {
		return Result{}, err
	}
	result := Result{Data: map[string]any{"path": path, "config": MaskConfig(cfg)}, Text: "created config at " + path}
	if req.NoPDF || (cfg.Mode != "local" && cfg.Mode != "hybrid" && cfg.Mode != "remote") || cfg.DataDir == "" {
		return result, nil
	}
	wantPDF := req.SetupPDF
	if !wantPDF && !nonInteractive && req.ConfirmPDF != nil {
		wantPDF, err = req.ConfirmPDF()
		if err != nil {
			return Result{}, err
		}
	}
	if !wantPDF {
		return result, nil
	}
	pdfResult, err := setupPDFResult(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	result.Meta = pdfResult.Meta
	result.Text += "\n" + pdfResult.Text
	return result, nil
}

func checkPDFResult(cfg config.Config) (Result, error) {
	if cfg.DataDir == "" {
		return Result{}, fmt.Errorf("ZOT_DATA_DIR is required; run 'zot config init' first")
	}
	status := backend.CheckVenvStatus(cfg.DataDir)
	data := map[string]any{"data_dir": cfg.DataDir, "venv_path": status.VenvPath, "python_path": status.PythonPath,
		"has_uv": status.HasUV, "has_pymupdf": status.HasPyMuPDF, "setup_needed": status.SetupNeeded, "error": status.Error}
	text := fmt.Sprintf("PDF extraction: pymupdf=%t python=%s", status.HasPyMuPDF, status.PythonPath)
	return Result{Data: data, Text: text}, nil
}

func setupPDFResult(ctx context.Context, cfg config.Config) (Result, error) {
	if cfg.Mode != "local" && cfg.Mode != "hybrid" && cfg.Mode != "remote" {
		return Result{Data: map[string]any{"skipped": true}, Text: "PyMuPDF setup skipped in web mode", Warnings: []Warning{{Code: "pdf_setup_ignored", Message: "--pdf flag has no effect in web mode; PyMuPDF is only used for local/hybrid modes"}}}, nil
	}
	if cfg.DataDir == "" {
		return Result{}, fmt.Errorf("ZOT_DATA_DIR is required for PyMuPDF setup")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := backend.SetupVenv(ctx, cfg.DataDir); err != nil {
		return Result{}, fmt.Errorf("PyMuPDF setup failed: %w", err)
	}
	status := backend.CheckVenvStatus(cfg.DataDir)
	if !status.HasPyMuPDF {
		return Result{}, fmt.Errorf("setup completed but PyMuPDF verification failed")
	}
	return Result{Data: map[string]any{"python_path": status.PythonPath, "has_pymupdf": true}, Text: "PyMuPDF setup complete. Python: " + status.PythonPath}, nil
}

func (ConfigService) Show(_ context.Context, pathOnly bool) (Result, error) {
	if pathOnly {
		path, err := config.DefaultPath()
		if err != nil {
			return Result{}, err
		}
		return Result{Data: map[string]any{"path": path}, Text: path}, nil
	}
	cfg, path, err := config.Load()
	if err != nil {
		return Result{}, err
	}
	data := map[string]any{"path": path, "config": MaskConfig(cfg)}
	return Result{Data: data, Text: formatConfigText(path, cfg)}, nil
}

func (ConfigService) Check(ctx context.Context) (Result, error) {
	cfg, path, err := config.Load()
	if err != nil {
		return Result{}, err
	}
	client := zoteroapi.New(cfg, os.Getenv("ZOT_BASE_URL"), nil)
	access, err := client.ValidateLibraryAccess(ctx)
	if err != nil {
		return Result{}, err
	}
	meta := ConfigCheckMeta(cfg, path)
	return Result{Data: access, Meta: meta, Text: "configuration is valid"}, nil
}

func ConfigCheckMeta(cfg config.Config, path string) map[string]any {
	meta := map[string]any{"config_path": path, "mode": cfg.Mode, "data_dir_configured": cfg.DataDir != ""}
	if cfg.DataDir == "" {
		return meta
	}
	if _, err := backend.NewLocalReader(cfg); err != nil {
		meta["local_reader_available"] = false
		meta["local_reader_error"] = err.Error()
		return meta
	}
	meta["local_reader_available"] = true
	return meta
}

func MaskConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"mode": cfg.Mode, "data_dir": cfg.DataDir, "library_type": cfg.LibraryType,
		"library_id": cfg.LibraryID, "api_key": maskSecret(cfg.APIKey), "server_addr": cfg.ServerAddr,
		"server_auth_key": maskSecret(cfg.ServerAuthKey), "style": cfg.Style, "locale": cfg.Locale,
		"timeout_seconds": cfg.TimeoutSeconds, "retry_max_attempts": cfg.RetryMaxAttempts,
		"retry_base_delay_ms": cfg.RetryBaseDelayMilliseconds, "allow_write": cfg.AllowWrite,
		"allow_delete": cfg.AllowDelete,
	}
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

func formatConfigText(path string, cfg config.Config) string {
	return "config: " + path + "\nmode: " + cfg.Mode + "\nlibrary: " + cfg.LibraryType + "/" + cfg.LibraryID
}

func IsConfigNotFound(err error) bool { return errors.Is(err, config.ErrNotFound) }
