package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type LoggerKey struct{}
type RequestIDKey struct{}

func RequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := uuid.NewString()
			logger := base.With(
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
			w.Header().Set("X-Request-ID", reqID)
			ctx := context.WithValue(r.Context(), LoggerKey{}, logger)
			ctx = context.WithValue(ctx, RequestIDKey{}, reqID)
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK, logger: logger, requestID: reqID}
			start := time.Now()
			next.ServeHTTP(rec, r.WithContext(ctx))
			logger.Info("request_completed",
				slog.Int("status_code", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, _ := ctx.Value(LoggerKey{}).(*slog.Logger)
	if logger == nil {
		return slog.Default()
	}
	return logger
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(RequestIDKey{}).(string)
	return requestID
}

type statusRecorder struct {
	http.ResponseWriter
	status    int
	logger    *slog.Logger
	requestID string
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Logger() *slog.Logger {
	return s.logger
}

func (s *statusRecorder) RequestID() string {
	return s.requestID
}
