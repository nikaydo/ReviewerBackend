package database

import (
	"context"
	"fmt"
	"time"
)

// DefaultReviewModel — модель по умолчанию для генерации отзывов.
const DefaultReviewModel = "anthropic/claude-sonnet-4.6"

// Review — сохранённый отзыв.
type Review struct {
	ID        string    `json:"uuid"`
	UserID    string    `json:"user"`
	Request   string    `json:"request"`
	Answer    string    `json:"answer"`
	Reasoning string    `json:"think"`
	CreatedAt time.Time `json:"date"`
	Model     string    `json:"model"`
	Favorite  bool      `json:"favorite"`
	Title     Title     `json:"title"`
}

// Title — заголовок отзыва, сгенерированный моделью.
type Title struct {
	ID       string  `json:"uuid,omitempty"`
	Request  *string `json:"request"`
	Text     *string `json:"title"`
	ReviewID string  `json:"uuidreview"`
}

// CreateReview сохраняет отзыв.
func (s *Store) CreateReview(ctx context.Context, r Review) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO reviews (uuid, request, answer, reasoning, model, favorite, date)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, now()))
		RETURNING uuid_uniq::text
	`, r.UserID, r.Request, r.Answer, r.Reasoning, r.Model, r.Favorite, nullableTime(r.CreatedAt)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить отзыв: %w", err)
	}
	return id, nil
}

// ListReviews возвращает все отзывы пользователя вместе с заголовками.
//
// Запрос разбит на два, а не использует JOIN с LEFT JOIN по нескольким
// заголовкам: в исходной версии один отзыв мог возвращаться несколько раз
// при нескольких заголовках на него.
func (s *Store) ListReviews(ctx context.Context, userID string) ([]Review, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT uuid_uniq::text,
		       uuid::text,
		       request,
		       answer,
		       reasoning,
		       date,
		       model,
		       favorite
		FROM reviews
		WHERE uuid = $1
		ORDER BY date DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить отзывы: %w", err)
	}
	defer rows.Close()

	reviews := make([]Review, 0)
	index := make(map[string]int)

	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.UserID, &r.Request, &r.Answer, &r.Reasoning,
			&r.CreatedAt, &r.Model, &r.Favorite); err != nil {
			return nil, fmt.Errorf("ошибка чтения отзыва: %w", err)
		}
		reviews = append(reviews, r)
		index[r.ID] = len(reviews) - 1
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка перебора отзывов: %w", err)
	}

	if len(reviews) == 0 {
		return reviews, nil
	}

	titles, err := s.titlesFor(ctx, userID, reviews)
	if err != nil {
		return nil, err
	}
	for id, t := range titles {
		if i, ok := index[id]; ok {
			reviews[i].Title = t
		}
	}
	return reviews, nil
}

// titlesFor загружает заголовки для переданных отзывов.
func (s *Store) titlesFor(ctx context.Context, userID string, reviews []Review) (map[string]Title, error) {
	ids := make([]string, 0, len(reviews))
	for _, r := range reviews {
		ids = append(ids, r.ID)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT uuid::text, title, request, uuid_review::text
		FROM review_titles
		WHERE uuid_user = $1 AND uuid_review = ANY($2::uuid[])
	`, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить заголовки: %w", err)
	}
	defer rows.Close()

	out := make(map[string]Title, len(reviews))
	for rows.Next() {
		var t Title
		if err := rows.Scan(&t.ID, &t.Text, &t.Request, &t.ReviewID); err != nil {
			return nil, fmt.Errorf("ошибка чтения заголовка: %w", err)
		}
		out[t.ReviewID] = t
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка перебора заголовков: %w", err)
	}
	return out, nil
}

// ReviewByID возвращает отзыв пользователя.
//
// userID передаётся обязательным параметром и участвует в WHERE: так запрос
// нельзя выполнить для чужого отзыва, даже зная его идентификатор.
func (s *Store) ReviewByID(ctx context.Context, userID, reviewID string) (Review, error) {
	var r Review
	err := s.pool.QueryRow(ctx, `
		SELECT uuid_uniq::text, uuid::text, request, answer, reasoning, date, model, favorite
		FROM reviews
		WHERE uuid = $1 AND uuid_uniq = $2
	`, userID, reviewID).Scan(&r.ID, &r.UserID, &r.Request, &r.Answer, &r.Reasoning,
		&r.CreatedAt, &r.Model, &r.Favorite)
	if err != nil {
		if isNoRows(err) {
			return Review{}, ErrNotFound
		}
		return Review{}, fmt.Errorf("не удалось получить отзыв: %w", err)
	}
	return r, nil
}

// UpdateReviewAnswer перезаписывает текст отзыва.
func (s *Store) UpdateReviewAnswer(ctx context.Context, userID, reviewID, answer string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE reviews SET answer = $1 WHERE uuid = $2 AND uuid_uniq = $3`,
		answer, userID, reviewID)
	if err != nil {
		return fmt.Errorf("не удалось обновить отзыв: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetReviewFavorite помечает отзыв избранным.
func (s *Store) SetReviewFavorite(ctx context.Context, userID, reviewID string, favorite bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE reviews SET favorite = $1 WHERE uuid = $2 AND uuid_uniq = $3`,
		favorite, userID, reviewID)
	if err != nil {
		return fmt.Errorf("не удалось изменить избранное: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteReview удаляет отзыв вместе с заголовком.
func (s *Store) DeleteReview(ctx context.Context, userID, reviewID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM review_titles WHERE uuid_user = $1 AND uuid_review = $2`, userID, reviewID); err != nil {
		return fmt.Errorf("не удалось удалить заголовок: %w", err)
	}
	tag, err := tx.Exec(ctx,
		`DELETE FROM reviews WHERE uuid = $1 AND uuid_uniq = $2`, userID, reviewID)
	if err != nil {
		return fmt.Errorf("не удалось удалить отзыв: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("не удалось зафиксировать транзакцию: %w", err)
	}
	return nil
}

// UpsertReviewTitle сохраняет заголовок отзыва.
func (s *Store) UpsertReviewTitle(ctx context.Context, userID, reviewID, title, request string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO review_titles (uuid_user, uuid_review, title, request)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (uuid_user, uuid_review) DO UPDATE
		SET title = EXCLUDED.title,
		    request = EXCLUDED.request
	`, userID, reviewID, title, request)
	if err != nil {
		return fmt.Errorf("не удалось сохранить заголовок: %w", err)
	}
	return nil
}

// UpdateReviewTitle меняет заголовок на заданный пользователем.
func (s *Store) UpdateReviewTitle(ctx context.Context, userID, reviewID, title string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE review_titles SET title = $1 WHERE uuid_user = $2 AND uuid_review = $3`,
		title, userID, reviewID)
	if err != nil {
		return fmt.Errorf("не удалось обновить заголовок: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReviewTitle возвращает заголовок отзыва.
func (s *Store) ReviewTitle(ctx context.Context, userID, reviewID string) (Title, error) {
	var t Title
	err := s.pool.QueryRow(ctx, `
		SELECT uuid::text, request, title, uuid_review::text
		FROM review_titles
		WHERE uuid_user = $1 AND uuid_review = $2
	`, userID, reviewID).Scan(&t.ID, &t.Request, &t.Text, &t.ReviewID)
	if err != nil {
		if isNoRows(err) {
			return Title{}, ErrNotFound
		}
		return Title{}, fmt.Errorf("не удалось получить заголовок: %w", err)
	}
	return t, nil
}

// nullableTime возвращает nil для нулевого времени, чтобы сработал
// COALESCE($7, now()) в SQL.
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
