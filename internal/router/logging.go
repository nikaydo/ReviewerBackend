package router

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// newRequestID генерирует идентификатор запроса.
func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

// statusRecorder запоминает код ответа для журналирования.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader запоминает код и передаёт его дальше.
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write считает объём ответа и передаёт его дальше.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// requestLogger пишет по одной структурированной записи на запрос.
//
// Раньше использовался middleware.Logger из chi: он печатал каждую строку
// запроса в текстовом виде, что неудобно искать в логах. Собственный
// middleware пишет JSON и добавляет идентификатор запроса и длительность.
func requestLogger(next http.Handler) http.Handler {
	log := slog.Default()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 0}

		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}

		log.LogAttrs(r.Context(), level, "http_request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", requestIDFrom(r.Context())),
		)
	})
}

// requestIDMiddleware присваивает запросу идентификатор и кладёт его
// в контекст.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := chimw.GetReqID(r.Context())
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), id)))
	})
}
