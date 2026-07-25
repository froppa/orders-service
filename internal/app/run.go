package app

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

func (a *App) Run(ctx context.Context) error {
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	serverErr := make(chan error, 1)
	workerErr := make(chan error, 1)

	go func() {
		if err := a.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	go func() {
		err := a.Worker.Run(workerCtx)
		if err != nil && !errors.Is(err, context.Canceled) {
			workerErr <- err
			return
		}
		workerErr <- nil
	}()

	a.Logger.Info("orders-service starting", zap.String("addr", a.Config.HTTPAddr))

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case err := <-workerErr:
		if err != nil {
			return err
		}
	}

	workerCancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.ShutdownTimeout)
	defer cancel()
	a.Logger.Info("orders-service shutting down")
	serverShutdownErr := a.HTTPServer.Shutdown(shutdownCtx)
	otelShutdownErr := a.ShutdownOTEL(shutdownCtx)
	if serverShutdownErr != nil {
		return serverShutdownErr
	}
	return otelShutdownErr
}
