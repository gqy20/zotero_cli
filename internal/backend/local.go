package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"zotero_cli/internal/config"
	"zotero_cli/internal/domain"
)

type LocalReader struct {
	DataDir    string
	SQLitePath string
	StorageDir string
}

func NewLocalReader(cfg config.Config) (*LocalReader, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("local mode requires data_dir")
	}

	dataDir, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	sqlitePath := filepath.Join(dataDir, "zotero.sqlite")
	storageDir := filepath.Join(dataDir, "storage")
	if err := requireDir(dataDir, "data_dir"); err != nil {
		return nil, err
	}
	if err := requireFile(sqlitePath, "zotero.sqlite"); err != nil {
		return nil, err
	}
	if err := requireDir(storageDir, "storage"); err != nil {
		return nil, err
	}

	return &LocalReader{
		DataDir:    dataDir,
		SQLitePath: sqlitePath,
		StorageDir: storageDir,
	}, nil
}

func (r *LocalReader) FindItems(ctx context.Context, opts FindOptions) ([]domain.Item, error) {
	return nil, fmt.Errorf("local find is not implemented yet")
}

func (r *LocalReader) GetItem(ctx context.Context, key string) (domain.Item, error) {
	return domain.Item{}, fmt.Errorf("local show is not implemented yet")
}

func requireDir(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %s", label, path)
	}
	return nil
}

func requireFile(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is not a file: %s", label, path)
	}
	return nil
}
