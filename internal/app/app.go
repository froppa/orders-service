package app

import (
	"context"
	"net/http"

	"github.com/froppa/orders-service/internal/config"
	infraDB "github.com/froppa/orders-service/internal/infrastructure/db"
	"github.com/froppa/orders-service/internal/infrastructure/worker"
	"go.uber.org/zap"
)

type App struct {
	Config       config.Config
	Logger       *zap.Logger
	DB           *infraDB.DB
	HTTPServer   *http.Server
	Worker       *worker.Worker
	ShutdownOTEL func(context.Context) error
}

func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

func (a *App) Migrate(ctx context.Context) error {
	return a.DB.Migrate(ctx)
}
