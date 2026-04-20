package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type VenvStatus struct {
	VenvPath    string
	PythonPath  string
	HasPyMuPDF  bool
	HasUV       bool
	SetupNeeded bool
	Error       string
}

var findPythonCommandFunc = func(dataDir string) (string, bool) {
	return findPythonCommand(dataDir)
}

func pythonVenvDir(dataDir string) string {
	return filepath.Join(dataDir, ".zotero_cli", "venv")
}

func pythonVenvExecutable(venvDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "python.exe")
	}
	return filepath.Join(venvDir, "bin", "python")
}

func findPythonCommand(dataDir string) (string, bool) {
	venvDir := pythonVenvDir(dataDir)
	venvPython := pythonVenvExecutable(venvDir)
	if _, err := exec.LookPath(venvPython); err == nil {
		return venvPython, true
	}

	for _, candidate := range []string{"python3", "python"} {
		if path, err := exec.LookPath(candidate); err == nil {
			if checkPythonHasFitz(path) {
				return path, true
			}
		}
	}

	return "", false
}

func checkPythonHasFitz(pythonPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonPath, "-c", "import fitz")
	err := cmd.Run()
	return err == nil
}

func lookPathUV() (string, bool) {
	if path, err := exec.LookPath("uv"); err == nil {
		return path, true
	}
	return "", false
}

func CheckVenvStatus(dataDir string) VenvStatus {
	status := VenvStatus{
		VenvPath: pythonVenvDir(dataDir),
	}

	if uvPath, ok := lookPathUV(); ok {
		status.HasUV = true
		_ = uvPath
	}

	venvPython := pythonVenvExecutable(status.VenvPath)
	if _, err := os.Stat(venvPython); err == nil {
		status.PythonPath = venvPython
		status.HasPyMuPDF = checkPythonHasFitz(venvPython)
		if !status.HasPyMuPDF {
			status.SetupNeeded = true
		}
		return status
	}

	for _, candidate := range []string{"python3", "python"} {
		if path, err := exec.LookPath(candidate); err == nil {
			status.PythonPath = path
			status.HasPyMuPDF = checkPythonHasFitz(path)
			break
		}
	}

	status.SetupNeeded = true
	return status
}

func SetupVenv(ctx context.Context, dataDir string) error {
	venvDir := pythonVenvDir(dataDir)

	parentDir := filepath.Dir(venvDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("create .zotero_cli dir: %w", err)
	}

	if _, err := os.Stat(venvDir); err == nil {
		if err := os.RemoveAll(venvDir); err != nil {
			return fmt.Errorf("remove existing venv: %w", err)
		}
	}

	if uvPath, ok := lookPathUV(); ok {
		return setupWithUV(ctx, uvPath, venvDir)
	}

	return setupWithSystemPython(ctx, venvDir)
}

func setupWithUV(ctx context.Context, uvPath, venvDir string) error {
	cmd := exec.CommandContext(ctx, uvPath, "venv", venvDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uv venv failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	venvPython := pythonVenvExecutable(venvDir)
	pipCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(pipCtx, uvPath, "pip", "install",
		"--python", venvPython, "pymupdf")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uv pip install pymupdf failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func setupWithSystemPython(ctx context.Context, venvDir string) error {
	var systemPython string
	for _, candidate := range []string{"python3", "python"} {
		if p, err := exec.LookPath(candidate); err == nil {
			systemPython = p
			break
		}
	}
	if systemPython == "" {
		return fmt.Errorf("no python found on PATH; install Python or uv")
	}

	cmd := exec.CommandContext(ctx, systemPython, "-m", "venv", venvDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("python -m venv failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	venvPython := pythonVenvExecutable(venvDir)
	pipCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd = exec.CommandContext(pipCtx, venvPython, "-m", "pip", "install", "pymupdf")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pip install pymupdf failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
