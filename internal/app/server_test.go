package app

import (
	"context"
	"sync/atomic"
	"testing"

	"zotero_cli/internal/config"
)

func TestServerStartOwnsLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var stopped atomic.Bool
	service := NewServerService()
	service.LoadConfig = func() (config.Config, string, error) { return config.Config{Mode: "local"}, "", nil }
	service.Serve = func(config.Config) (func(), error) {
		cancel()
		return func() { stopped.Store(true) }, nil
	}
	if _, err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !stopped.Load() {
		t.Fatal("expected server shutdown")
	}
}

func TestServerStartRejectsRemoteMode(t *testing.T) {
	service := NewServerService()
	service.LoadConfig = func() (config.Config, string, error) { return config.Config{Mode: "remote"}, "", nil }
	if _, err := service.Start(context.Background()); err == nil {
		t.Fatal("expected remote mode error")
	}
}
