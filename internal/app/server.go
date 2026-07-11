package app

import (
	"context"
	"fmt"
	"os"

	"zotero_cli/internal/config"
	"zotero_cli/internal/server"
)

type ServerStartRequest struct {
	Port string
}

type ServerService struct {
	LoadConfig func() (config.Config, string, error)
	Serve      func(config.Config) (func(), error)
}

func NewServerService() ServerService {
	return ServerService{LoadConfig: config.Load, Serve: server.ServeFromConfig}
}

func (s ServerService) Start(ctx context.Context, req ServerStartRequest) (Result, error) {
	if req.Port != "" {
		previous, existed := os.LookupEnv("ZOT_SERVER_PORT")
		if err := os.Setenv("ZOT_SERVER_PORT", req.Port); err != nil {
			return Result{}, err
		}
		defer func() {
			if existed {
				_ = os.Setenv("ZOT_SERVER_PORT", previous)
			} else {
				_ = os.Unsetenv("ZOT_SERVER_PORT")
			}
		}()
	}
	cfg, _, err := s.LoadConfig()
	if err != nil {
		return Result{}, err
	}
	if cfg.Mode == "remote" {
		return Result{}, fmt.Errorf("server cannot start in remote mode; configure web, local, or hybrid mode")
	}
	shutdown, err := s.Serve(cfg)
	if err != nil {
		return Result{}, fmt.Errorf("start server: %w", err)
	}
	defer shutdown()
	<-ctx.Done()
	return Result{Data: map[string]any{"stopped": true}, Text: "server stopped"}, nil
}
