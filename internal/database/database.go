// Package database отвечает за работу с PostgreSQL.
//
// Имена таблиц больше не приходят из переменных окружения: раньше они
// подставлялись в SQL конкатенацией, что делало запросы зависимыми от
// конфигурации и исключало возможность использовать параметризованные
// подготовленные выражения. Теперь имена таблиц зафиксированы в коде, а
// все пользовательские данные передаются параметрами $1, $2, …
//
// Все методы принимают context.Context, чтобы отмена запроса клиента
// прерывала и HTTP-запрос, и обращение к базе.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ошибки уровня домена.
var (
	ErrNotFound     = errors.New("запись не найдена")
	ErrLoginTaken   = errors.New("логин уже занят")
	ErrWeakPassword = errors.New("пароль слишком простой")
	ErrInvalidLogin = errors.New("неверный логин или пароль")
)

// Store — обёртка над пулом соединений.
type Store struct {
	pool *pgxpool.Pool
}

// New открывает пул соединений и проверяет его работоспособность.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("не удалось разобрать строку подключения: %w", err)
	}
	// Ограничиваем пул: без этого pgx удерживает соединение максимум
	// до нескольких часов idle, что мешает перезапуску БД в оркестраторе.
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пул соединений: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("база данных недоступна: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close закрывает пул соединений.
func (s *Store) Close() {
	s.pool.Close()
}

// Pool возвращает пул для мест, где нужен прямой доступ (миграции).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// RunMigrations применяет миграции из каталога dir.
func RunMigrations(ctx context.Context, databaseURL, dir string) error {
	m, err := migrate.New("file://"+dir, databaseURL)
	if err != nil {
		return fmt.Errorf("не удалось инициализировать миграции: %w", err)
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil {
			fmt.Printf("ошибка закрытия источника миграций: %v\n", sourceErr)
		}
		if dbErr != nil {
			fmt.Printf("ошибка закрытия соединения миграций: %v\n", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("не удалось применить миграции: %w", err)
	}
	return nil
}

// isNoRows сообщает, что запрос не вернул строк.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
