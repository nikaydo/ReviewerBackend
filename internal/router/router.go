// Package router собирает HTTP-маршруты приложения.
package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/nikaydo/reviewer/internal/ai"
	"github.com/nikaydo/reviewer/internal/handlers"
)

// requestTimeoutValue ограничивает суммарное время обработки одного запроса.
const requestTimeoutValue = 30 * time.Second

// Params — параметры сборки маршрутов.
type Params struct {
	Handlers *handlers.Handlers
	// StaticDir — каталог со статикой. Если пуст, раздача отключается.
	StaticDir string
	// SecureHeaders включает строгие заголовки безопасности.
	SecureHeaders bool
}

// New собирает обработчик запросов.
func New(p Params) http.Handler {
	r := chi.NewRouter()

	r.Use(requestIDMiddleware)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))
	r.Use(requestLogger)
	r.Use(requestTimeout(requestTimeoutValue))

	if p.SecureHeaders {
		r.Use(securityHeaders)
	}

	h := p.Handlers

	// Публичная часть: только вход и регистрация под лимитом частоты.
	r.Group(func(r chi.Router) {
		r.Use(h.LimitAuth)

		r.Post("/api/signup", h.SignUp)
		r.Post("/api/signin", h.SignIn)
		// Обновление сессии не проходит через Authenticate: access-токен
		// к этому моменту уже истёк — именно ради этого эндпоинта он и нужен.
		r.Post("/api/refresh", h.Refresh)
		r.Get("/api/health", healthHandler)
		r.Get("/api/models", modelsHandler)
	})

	// Приватная часть: всё под проверкой access-токена.
	r.Group(func(r chi.Router) {
		r.Use(h.Authenticate)

		r.Post("/api/logout", h.Logout)
		r.Get("/api/me", h.Me)

		r.Route("/api/settings", func(r chi.Router) {
			r.Get("/", h.GetSettings)
			r.Put("/", h.UpdateSettings)
		})

		r.Route("/api/reviews", func(r chi.Router) {
			r.Get("/", h.ListReviews)
			r.Post("/", h.CreateReview)
			r.Put("/", h.UpdateReview)
			r.Delete("/{reviewID}", h.DeleteReview)
			r.Post("/favorite", h.Favorite)
		})

		r.Route("/api/titles", func(r chi.Router) {
			r.Post("/", h.CreateTitle)
			r.Put("/", h.UpdateTitle)
		})

		r.Post("/api/main-prompt", h.UpdateMainPrompt)

		r.Route("/api/memory", func(r chi.Router) {
			r.Get("/", h.GetMemory)
			r.Put("/", h.SetMemory)
		})

		r.Route("/api/prompts", func(r chi.Router) {
			r.Get("/", h.ListPrompts)
			r.Post("/", h.CreatePrompt)
			r.Put("/{promptID}", h.UpdatePrompt)
			r.Delete("/{promptID}", h.DeletePrompt)
		})

		r.Get("/api/queue/{jobID}", h.QueueStatus)
	})

	if p.StaticDir != "" {
		// Страницы отдаются по явным маршрутам, а не файловым сервером
		// «как есть»: иначе адрес приложения зависел бы от имени файла, а
		// запрошенный путь мог бы выйти за пределы каталога со статикой.
		files := http.Dir(p.StaticDir)

		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, filepath.Join(p.StaticDir, "index.html"))
		})
		r.Get("/app", func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, filepath.Join(p.StaticDir, "user.html"))
		})
		// Старое имя оставлено как синоним, чтобы старые закладки работали.
		r.Get("/user.html", func(w http.ResponseWriter, req *http.Request) {
			http.ServeFile(w, req, filepath.Join(p.StaticDir, "user.html"))
		})

		// Остальные файлы каталога отдаются файловым сервером.
		r.Handle("/*", http.FileServer(files))
	}

	return r
}

// healthHandler сообщает о готовности приложения.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// modelsHandler отдаёт список доступных моделей.
//
// Список формируется на сервере, чтобы клиент не мог запросить модель,
// которой нет в белом списке.
func modelsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, ai.Models)
}

// writeJSONResponse пишет ответ в JSON.
func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Default().Error("не удалось записать ответ", "err", err)
	}
}

// securityHeaders выставляет заголовки безопасности.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

// requestTimeout ограничивает время обработки запроса.
func requestTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
