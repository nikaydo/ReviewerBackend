package database

import (
	"context"
	"fmt"
	"time"
)

// RefreshToken — запись о выданном refresh-токене.
//
// Хранится только хеш токена. Поле RevokedAt позволяет отозвать конкретную
// пару «access/refresh», а FamilyID связывает все токены, выданные в рамках
// одного входа: при попытке использовать уже отозванный токен отзывается
// всё семейство. Это защита от повторного использования токена после
// утечки — стандартный приём в OAuth 2.0 и RFC 6819.
type RefreshToken struct {
	ID        string
	UserID    string
	FamilyID  string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// ActiveRefreshToken возвращает неотозванный и не истёкший токен семьи.
func (r *RefreshToken) Active(now time.Time) bool {
	return r.RevokedAt == nil && r.ExpiresAt.After(now)
}

// ErrRefreshReused возвращается, когда предъявлен уже отозванный токен.
//
// Отдельный тип нужен, чтобы middleware отличал повторное использование
// от обычного отказа и мог отозвать всё семейство.
type ErrRefreshReused struct {
	UserID   string
	FamilyID string
}

func (e *ErrRefreshReused) Error() string {
	return "refresh-токен уже использован, семейство токенов отозвано"
}

// SaveRefreshToken сохраняет хеш выданного refresh-токена.
func (s *Store) SaveRefreshToken(ctx context.Context, userID, familyID, tokenHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (uuid, user_uuid, family_id, token_hash, expires_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, userID, familyID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("не удалось сохранить refresh-токен: %w", err)
	}
	return nil
}

// RotateRefreshToken заменяет токен в семействе на новый.
//
// Ротация оформляется двумя операциями в одной транзакции: текущий токен
// помечается отозванным, новый добавляется отдельной записью. Именно так, а не
// обновлением поля token_hash, потому что иначе хеш старого токена
// перезаписывается и перестаёт существовать — повторное предъявление старого
// токена становится неотличимым от предъявления никогда не существовавшего,
// и защита от повторного использования не срабатывает.
//
// Возвращает ErrRefreshReused, если переданный токен уже отозван: значит,
// им воспользовались повторно, и всё семейство нужно аннулировать.
func (s *Store) RotateRefreshToken(ctx context.Context, userID, familyID, oldHash, newHash string, expiresAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Помечаем предъявленный токен отозванным. Условие revoked_at IS NULL
	// делает операцию идемпотентной: второй параллельный запрос с тем же
	// токеном не обновит ни одной строки и получит ErrRefreshReused.
	var rotatedID string
	err = tx.QueryRow(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now(), rotated_at = now()
		WHERE token_hash = $1
		  AND user_uuid = $2
		  AND family_id = $3
		  AND revoked_at IS NULL
		RETURNING uuid::text
	`, oldHash, userID, familyID).Scan(&rotatedID)
	if err != nil {
		if !isNoRows(err) {
			return fmt.Errorf("не удалось отозвать предыдущий токен: %w", err)
		}

		// Строка не обновилась. Различаем два случая: токен уже отозван
		// (признак утечки) или его не существует вовсе.
		var exists bool
		qErr := tx.QueryRow(ctx,
			`SELECT TRUE FROM refresh_tokens WHERE token_hash = $1 AND user_uuid = $2`,
			oldHash, userID).Scan(&exists)
		switch {
		case qErr != nil && !isNoRows(qErr):
			return fmt.Errorf("не удалось проверить состояние токена: %w", qErr)
		case exists:
			return &ErrRefreshReused{UserID: userID, FamilyID: familyID}
		default:
			return ErrNotFound
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (uuid, user_uuid, family_id, token_hash, expires_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4)
	`, userID, familyID, newHash, expiresAt); err != nil {
		return fmt.Errorf("не удалось сохранить новый токен: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
	}
	return nil
}

// UserByRefreshHash возвращает пользователя по хешу действующего refresh-токена.
//
// Ищется именно неотозванный и не истёкший токен: отозванный хеш тоже нужно
// находить, но уже для того, чтобы сообщить о повторном использовании, —
// это делает RotateRefreshToken.
func (s *Store) UserByRefreshHash(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT u.uuid::text, u.login, u.password_hash, u.role
		FROM refresh_tokens rt
		JOIN users u ON u.uuid = rt.user_uuid
		WHERE rt.token_hash = $1
		  AND rt.revoked_at IS NULL
		  AND rt.expires_at > now()
	`, tokenHash).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Role)
	if err != nil {
		if isNoRows(err) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("не удалось найти пользователя по токену: %w", err)
	}
	return u, nil
}

// RevokeRefreshToken отзывает конкретный токен.
func (s *Store) RevokeRefreshToken(ctx context.Context, userID, tokenHash string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND user_uuid = $2 AND revoked_at IS NULL
	`, tokenHash, userID)
	if err != nil {
		return fmt.Errorf("не удалось отозвать refresh-токен: %w", err)
	}
	return nil
}

// RevokeTokenFamily отзывает все токены семейства.
func (s *Store) RevokeTokenFamily(ctx context.Context, familyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE family_id = $1 AND revoked_at IS NULL
	`, familyID)
	if err != nil {
		return fmt.Errorf("не удалось отозвать семейство токенов: %w", err)
	}
	return nil
}

// RevokeAllUserTokens отзывает все refresh-токены пользователя.
//
// Вызывается при смене пароля: старые сессии перестают быть действительными.
func (s *Store) RevokeAllUserTokens(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_uuid = $1 AND revoked_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("не удалось отозвать токены пользователя: %w", err)
	}
	return nil
}

// CleanupExpiredTokens удаляет истёкшие токены старше retention.
//
// revoked_at пометки сохраняются в течение retention: их нужно держать,
// чтобы отличать повторное использование токена от неизвестного.
func (s *Store) CleanupExpiredTokens(ctx context.Context, retention time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < now() - $1::interval
	`, retention.String())
	if err != nil {
		return 0, fmt.Errorf("не удалось очистить истёкшие токены: %w", err)
	}
	return tag.RowsAffected(), nil
}
