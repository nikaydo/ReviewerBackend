// Command server — веб-приложение для генерации отзывов с помощью LLM.
//
// Запуск:
//
//	go run ./cmd/server
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nikaydo/reviewer/internal/ai"
	"github.com/nikaydo/reviewer/internal/config"
	"github.com/nikaydo/reviewer/internal/database"
	"github.com/nikaydo/reviewer/internal/handlers"
	"github.com/nikaydo/reviewer/internal/jwt"
	"github.com/nikaydo/reviewer/internal/queue"
	"github.com/nikaydo/reviewer/internal/router"
	"github.com/nikaydo/reviewer/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("приложение завершилось с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Info("конфигурация загружена", "addr", cfg.Addr(), "issuer", cfg.Issuer)

	// Контекст приложения отменяется по SIGINT/SIGTERM: по нему останавливаются
	// воркеры очереди и текущие запросы к базе.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := database.RunMigrations(startupCtx, cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return err
	}
	log.Info("миграции применены")

	store, err := database.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	client := ai.NewClient(
		cfg.OpenRouterAPIKey,
		cfg.OpenRouterBaseURL,
		cfg.OpenRouterReferer,
		cfg.OpenRouterTitle,
		cfg.OpenRouterTimeout,
		cfg.OpenRouterRetries,
	)

	q := queue.New(queue.Config{
		Workers:         cfg.QueueConcurrency,
		BetweenRequests: cfg.QueueBetweenRequests,
		Retention:       10 * time.Minute,
	}, client, store, nil, log)
	q.Start(ctx)

	tokens := jwt.NewManager(cfg.JWTSecret, cfg.Issuer, cfg.AccessTokenTTL)
	h := handlers.New(store, tokens, q, cfg, log)
	// Очереди нужен источник промта для памяти; он живёт в обработчиках,
	// поэтому цикл замыкается через ссылку, объявленную заранее.
	q.SetPromptProvider(h)

	handler := router.New(router.Params{
		Handlers:      h,
		StaticDir:     "web",
		SecureHeaders: cfg.CookieSecure,
	})

	srvCfg := server.Default(cfg.Addr())
	srv := server.New(srvCfg, handler)

	go cleanupExpiredTokens(ctx, store, log)

	err = server.Run(ctx, srv, srvCfg, log)

	// Очередь останавливается после HTTP-сервера: так текущие запросы
	// успевают завершиться, а воркеры — снять оставшиеся задания.
	q.Close()

	return err
}

// cleanupExpiredTokens удаляет истёкшие refresh-токены раз в сутки.
func cleanupExpiredTokens(ctx context.Context, store *database.Store, log *slog.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	const retention = 7 * 24 * time.Hour
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := store.CleanupExpiredTokens(ctx, retention)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Error("не удалось удалить истёкшие токены", "err", err)
				continue
			}
			if removed > 0 {
				log.Info("истёкшие токены удалены", "count", removed)
			}
		}
	}
}
