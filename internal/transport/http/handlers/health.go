package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/froppa/orders-service/internal/domain/orders"
	"github.com/froppa/orders-service/internal/observability"
)

type HealthHandler struct {
	logger *zap.Logger
	ready  func(context.Context) error
}

func NewHealthHandler(logger *zap.Logger, ready func(context.Context) error) *HealthHandler {
	return &HealthHandler{logger: logger, ready: ready}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.ready(r.Context()); err != nil {
		observability.WithContext(r.Context(), h.logger).Warn("readiness check failed", zap.Error(err))
		writeError(w, http.StatusServiceUnavailable, "not_ready", "service is not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"error":{"code":"internal_error","message":"failed to encode response"}}`)
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("request body must contain a single JSON document")
	}
	return nil
}

func mapDomainError(err error) (int, string, string) {
	switch {
	case errors.Is(err, orders.ErrInvalidCustomerID), errors.Is(err, orders.ErrInvalidAmount):
		return http.StatusBadRequest, "invalid_request", err.Error()
	case errors.Is(err, orders.ErrNotFound):
		return http.StatusNotFound, "not_found", err.Error()
	default:
		return http.StatusInternalServerError, "internal_error", "internal server error"
	}
}
