// Package handlers содержит HTTP-обработчики приложения.
package handlers

import (
	"context"
	"log/slog"

	"github.com/nikaydo/reviewer/internal/config"
	"github.com/nikaydo/reviewer/internal/database"
	"github.com/nikaydo/reviewer/internal/jwt"
	"github.com/nikaydo/reviewer/internal/queue"
)

// Handlers связывает зависимости обработчиков.
type Handlers struct {
	Store  *database.Store
	Tokens *jwt.Manager
	Queue  *queue.Queue
	Cfg    config.Config
	Log    *slog.Logger

	// authLimiter ограничивает частоту запросов к эндпоинтам
	// авторизации. Создаётся в New.
	authLimiter *limiter
}

// New создаёт набор обработчиков.
func New(store *database.Store, tokens *jwt.Manager, q *queue.Queue, cfg config.Config, log *slog.Logger) *Handlers {
	if log == nil {
		log = slog.Default()
	}
	return &Handlers{
		Store:       store,
		Tokens:      tokens,
		Queue:       q,
		Cfg:         cfg,
		Log:         log,
		authLimiter: newLimiter(cfg.AuthRateLimit, cfg.AuthRateWindow),
	}
}

// session — данные аутентифицированного пользователя в контексте запроса.
type session struct {
	UserID      string
	Login       string
	Role        string
	RefreshHash string
	FamilyID    string
}

type sessionKey struct{}

// withSession кладёт данные сессии в контекст запроса.
func withSession(ctx context.Context, s session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// sessionFrom извлекает данные сессии из контекста.
func sessionFrom(ctx context.Context) (session, bool) {
	s, ok := ctx.Value(sessionKey{}).(session)
	return s, ok
}
