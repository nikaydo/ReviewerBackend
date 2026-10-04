package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Ограничения на логин и пароль.
const (
	MinLoginLength    = 3
	MaxLoginLength    = 32
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

// User — пользователь системы.
type User struct {
	ID    string
	Login string
	Role  string
	// PasswordHash — bcrypt-хеш пароля.
	PasswordHash string
	// HasLegacyPassword сообщает, что пароль ещё хранится открытым текстом
	// и должен быть переведён в хеш при следующем входе.
	HasLegacyPassword bool
}

// CreateUserParams — данные для регистрации.
type CreateUserParams struct {
	Login        string
	PasswordHash string
	Role         string
	// DefaultPrompt и MemorizationPrompt переносятся в настройки пользователя,
	// чтобы каждый новый сразу получал рабочую конфигурацию генерации.
	DefaultPrompt string
	MemoryPrompt  string
}

// NormalizeLogin приводит логин к каноническому виду: без пробелов по краям
// и в нижнем регистре.
//
// Нормализация обязательна, потому что в базе стоит ограничение
// login = lower(login) и уникальный индекс по lower(login): без неё
// регистрация «User» падала бы с невнятной ошибкой ограничения, а «user»
// и «User» считались бы разными учётными записями.
func NormalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

// ValidateLogin проверяет логин.
func ValidateLogin(login string) error {
	normalized := NormalizeLogin(login)
	if len([]rune(normalized)) < MinLoginLength {
		return fmt.Errorf("логин должен содержать минимум %d символа", MinLoginLength)
	}
	if len([]rune(normalized)) > MaxLoginLength {
		return fmt.Errorf("логин не должен превышать %d символов", MaxLoginLength)
	}
	if strings.ContainsAny(normalized, " \t\n\r") {
		return errors.New("логин не должен содержать пробелов")
	}
	return nil
}

// ValidatePassword проверяет пароль.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("пароль должен содержать минимум %d символов", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		// bcrypt всё равно учитывает только первые 72 байта, поэтому
		// ограничение сверху защищает от молчаливого обрезания.
		return fmt.Errorf("пароль не должен превышать %d символов", MaxPasswordLength)
	}
	return nil
}

// CreateUser регистрирует пользователя вместе с настройками по умолчанию.
//
// Пользователь и его настройки создаются одной транзакцией: иначе при сбое
// на второй вставке остаётся пользователь без настроек, и генерация
// падает при первом же запросе.
func (s *Store) CreateUser(ctx context.Context, p CreateUserParams) (string, error) {
	if err := ValidateLogin(p.Login); err != nil {
		return "", err
	}
	if strings.TrimSpace(p.PasswordHash) == "" {
		return "", errors.New("хеш пароля не может быть пустым")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (login, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING uuid::text
	`, NormalizeLogin(p.Login), p.PasswordHash, p.Role).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrLoginTaken
		}
		return "", fmt.Errorf("не удалось создать пользователя: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_settings (uuid, main_prompt, memory_prompt, model, memory)
		VALUES ($1, $2, $3, $4, FALSE)
	`, id, p.DefaultPrompt, p.MemoryPrompt, DefaultReviewModel)
	if err != nil {
		return "", fmt.Errorf("не удалось создать настройки пользователя: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
	}
	return id, nil
}

// UserByLogin возвращает пользователя по логину.
func (s *Store) UserByLogin(ctx context.Context, login string) (User, error) {
	var u User
	var passwordHash string

	err := s.pool.QueryRow(ctx, `
		SELECT uuid::text, login, password_hash, role
		FROM users
		WHERE login = $1
	`, NormalizeLogin(login)).Scan(&u.ID, &u.Login, &passwordHash, &u.Role)
	if err != nil {
		if isNoRows(err) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("не удалось получить пользователя: %w", err)
	}

	u.PasswordHash = passwordHash
	u.HasLegacyPassword = !strings.HasPrefix(passwordHash, "$2")
	return u, nil
}

// UserByID возвращает пользователя по идентификатору.
func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT uuid::text, login, password_hash, role
		FROM users
		WHERE uuid = $1
	`, id).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Role)
	if err != nil {
		if isNoRows(err) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("не удалось получить пользователя: %w", err)
	}
	u.HasLegacyPassword = !strings.HasPrefix(u.PasswordHash, "$2")
	return u, nil
}

// UpdatePasswordHash заменяет хеш пароля пользователя.
func (s *Store) UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE uuid = $2`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("не удалось обновить пароль: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser удаляет пользователя вместе с его данными.
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE uuid = $1`, userID)
	if err != nil {
		return fmt.Errorf("не удалось удалить пользователя: %w", err)
	}
	return nil
}

// isUniqueViolation сообщает, что нарушено ограничение уникальности.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
