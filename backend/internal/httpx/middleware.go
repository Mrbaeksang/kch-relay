package httpx

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func Stack(logger *slog.Logger, timeout time.Duration) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := NewID()
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			r = r.WithContext(ctx)
			w.Header().Set("X-Request-ID", id)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			sw := &statusWriter{ResponseWriter: w}
			started := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("request panic", "request_id", id)
					if sw.status == 0 {
						WriteError(sw, r, Error(500, "INTERNAL_ERROR", "처리 중 오류가 발생했습니다."))
					}
				}
				route := "unmatched"
				if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
					route = rc.RoutePattern()
				}
				logger.Info("http request", "request_id", id, "method", r.Method, "route", route, "status", sw.status, "duration_ms", time.Since(started).Milliseconds())
			}()
			if media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); strings.HasPrefix(r.URL.Path, "/api/") && media == "application/json" {
				if r.ContentLength > MaxJSONBytes {
					WriteError(sw, r, Invalid("요청 본문이 너무 큽니다."))
					return
				}
				r.Body = http.MaxBytesReader(sw, r.Body, MaxJSONBytes)
			}
			next.ServeHTTP(sw, r)
		})
	}
}
