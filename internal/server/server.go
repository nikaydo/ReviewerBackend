// Package server запускает HTTP-сервер с корректным завершением работы.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Config — параметры HTTP-сервера.
type Config struct {
	Addr string
	// ReadHeaderTimeout защищает от медленных заголовков: без него
	// клиент может держать соединение, не отправив ничего.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	// MaxHeaderBytes ограничивает размер заголовков запроса.
	MaxHeaderBytes int
	// ShutdownTimeout — сколько ждать завершения активных запросов.
	ShutdownTimeout time.Duration
}

// Default возвращает разумные значения таймаутов.
func Default(addr string) Config {
	return Config{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Запись ограничена отдельным таймаутом: генерация отзыва идёт
		// через очередь и возвращается быстро, но не мгновенно.
		WriteTimeout:    60 * time.Second,
		IdleTimeout:     120 * time.Second,
		MaxHeaderBytes:  1 << 20,
		ShutdownTimeout: 30 * time.Second,
	}
}

// New собирает http.Server.
func New(cfg Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}

// Run запускает сервер и останавливает его по сигналу.
//
// В исходной версии весь код корректного завершения был закомментирован:
// процесс убивался по Ctrl+C, не дожидаясь завершения активных запросов,
// а очередь оставалась с незавершёнными заданиями.
func Run(ctx context.Context, srv *http.Server, cfg Config, log *slog.Logger) error {
	errCh := make(chan error, 1)

	go func() {
		log.Info("сервер запущен", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("сервер остановился с ошибкой: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("получен сигнал завершения, останавливаю сервер")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("сервер не завершился корректно: %w", err)
	}
	log.Info("сервер остановлен")

	return <-errCh
}
