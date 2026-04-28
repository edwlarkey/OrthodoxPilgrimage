package app

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// responseWriter is a wrapper around http.ResponseWriter that captures the
// status code and the size of the response.
type responseWriter struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// LoggingMiddleware logs each request using slog.
func (a *Application) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		ip := getRealIP(r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"ip", ip,
			"status", rw.status,
			"size", rw.written,
			"duration", time.Since(start),
			"user_agent", r.UserAgent(),
		)
	})
}

func getRealIP(r *http.Request) string {
	// Cloudflare
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	// Standard proxy header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can be a comma-separated list. The first one is the client.
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// Fallback to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		return r.RemoteAddr
	}
	return ip
}
