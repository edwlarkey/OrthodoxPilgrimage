package app

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRealIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name: "Cloudflare header",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.1",
			},
			expected: "203.0.113.1",
		},
		{
			name: "X-Forwarded-For single",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.2",
			},
			expected: "203.0.113.2",
		},
		{
			name: "X-Forwarded-For multiple",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.3, 192.0.2.1",
			},
			expected: "203.0.113.3",
		},
		{
			name:     "RemoteAddr fallback",
			remote:   "203.0.113.4:12345",
			expected: "203.0.113.4",
		},
		{
			name:     "RemoteAddr fallback no port",
			remote:   "203.0.113.5",
			expected: "203.0.113.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.remote != "" {
				req.RemoteAddr = tt.remote
			}
			assert.Equal(t, tt.expected, getRealIP(req))
		})
	}
}

func TestResponseWriter(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr, status: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte("hello"))

	assert.Equal(t, http.StatusCreated, rw.status)
	assert.Equal(t, int64(5), rw.written)
	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "hello", rr.Body.String())

	// Test writing without explicit WriteHeader
	rr2 := httptest.NewRecorder()
	rw2 := &responseWriter{ResponseWriter: rr2, status: http.StatusOK}
	rw2.Write([]byte("world"))
	assert.Equal(t, http.StatusOK, rw2.status)
	assert.Equal(t, int64(5), rw2.written)
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	app := &Application{}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("test"))
	})

	middleware := app.LoggingMiddleware(nextHandler)

	req, _ := http.NewRequest("GET", "/test-path", nil)
	req.Header.Set("User-Agent", "TestAgent")
	req.RemoteAddr = "1.2.3.4:5678"
	rr := httptest.NewRecorder()

	// Use a custom logger for this test to capture output
	oldDefault := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldDefault)

	middleware.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.Equal(t, "test", rr.Body.String())

	logOutput := buf.String()
	assert.Contains(t, logOutput, "\"msg\":\"request\"")
	assert.Contains(t, logOutput, "\"method\":\"GET\"")
	assert.Contains(t, logOutput, "\"path\":\"/test-path\"")
	assert.Contains(t, logOutput, "\"ip\":\"1.2.3.4\"")
	assert.Contains(t, logOutput, "\"status\":202")
	assert.Contains(t, logOutput, "\"user_agent\":\"TestAgent\"")
}
