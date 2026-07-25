package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/froppa/orders-service/internal/application/ports"
	"github.com/froppa/orders-service/internal/observability"
	"go.uber.org/zap"
)

type snapshotWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *snapshotWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *snapshotWriter) Write(p []byte) (int, error) {
	w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func Idempotency(repo ports.IdempotencyRepository, transactor ports.Transactor, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				writeJSONError(w, http.StatusBadRequest, "missing_idempotency_key", "Idempotency-Key header is required", nil)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid_body", "failed to read request body", nil)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			hashBytes := sha256.Sum256(body)
			requestHash := hex.EncodeToString(hashBytes[:])

			record := ports.IdempotencyRecord{
				Key:          key,
				RequestHash:  requestHash,
				ResponseCode: 0,
				ResponseBody: []byte(`{}`),
				CreatedAt:    time.Now().UTC(),
			}

			var reserved bool
			if err := transactor.WithinTx(r.Context(), func(ctx context.Context, tx ports.DBTX) error {
				var reserveErr error
				reserved, reserveErr = repo.Reserve(ctx, tx, record)
				return reserveErr
			}); err != nil {
				observability.WithContext(r.Context(), logger).Error("reserve idempotency key failed", zap.Error(err))
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "failed to reserve idempotency key", nil)
				return
			}

			if !reserved {
				var existing ports.IdempotencyRecord
				if err := transactor.WithinTx(r.Context(), func(ctx context.Context, tx ports.DBTX) error {
					var getErr error
					existing, getErr = repo.Get(ctx, tx, key)
					return getErr
				}); err != nil {
					observability.WithContext(r.Context(), logger).Error("load idempotency key failed", zap.Error(err))
					writeJSONError(w, http.StatusInternalServerError, "internal_error", "failed to load idempotency state", nil)
					return
				}
				if existing.RequestHash != requestHash {
					writeJSONError(w, http.StatusConflict, "idempotency_conflict", "idempotency key reused with different request", nil)
					return
				}
				if existing.ResponseCode == 0 {
					writeJSONError(w, http.StatusConflict, "request_in_progress", "request is already in progress", nil)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Idempotency-Replayed", "true")
				w.WriteHeader(existing.ResponseCode)
				_, _ = w.Write(existing.ResponseBody)
				return
			}

			recorder := &snapshotWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)

			responseBody := recorder.body.Bytes()
			if len(responseBody) == 0 {
				responseBody = []byte(`{}`)
			}
			if err := transactor.WithinTx(r.Context(), func(ctx context.Context, tx ports.DBTX) error {
				return repo.Finalize(ctx, tx, key, recorder.status, responseBody)
			}); err != nil {
				observability.WithContext(r.Context(), logger).Error("finalize idempotency key failed", zap.Error(err))
			}
		})
	}
}
