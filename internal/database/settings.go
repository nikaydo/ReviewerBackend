package database

import (
	"context"
	"fmt"
)

// Settings — настройки генерации пользователя.
type Settings struct {
	UserID         string  `json:"uuid,omitempty"`
	MainPrompt     *string `json:"mainpromt"`
	MemoryPrompt   *string `json:"memoryprompt"`
	Request        string  `json:"request"`
	Model          string  `json:"model"`
	UseMemory      bool    `json:"memory"`
	InProgress     string  `json:"inprogress"`
	ProcessedCount int     `json:"count"`
}

// SettingsByUser возвращает настройки пользователя.
func (s *Store) SettingsByUser(ctx context.Context, userID string) (Settings, error) {
	var st Settings
	err := s.pool.QueryRow(ctx, `
		SELECT uuid::text, main_prompt, memory_prompt, request, model, memory,
		       COALESCE(inprogress, ''), COALESCE(processed_count, 0)
		FROM user_settings
		WHERE uuid = $1
	`, userID).Scan(&st.UserID, &st.MainPrompt, &st.MemoryPrompt, &st.Request,
		&st.Model, &st.UseMemory, &st.InProgress, &st.ProcessedCount)
	if err != nil {
		if isNoRows(err) {
			return Settings{}, ErrNotFound
		}
		return Settings{}, fmt.Errorf("не удалось получить настройки: %w", err)
	}
	return st, nil
}

// UpdateSettings сохраняет настройки пользователя.
func (s *Store) UpdateSettings(ctx context.Context, st Settings) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE user_settings
		SET main_prompt = $1, request = $2, model = $3, memory = $4, processed_count = $5
		WHERE uuid = $6
	`, st.MainPrompt, st.Request, st.Model, st.UseMemory, st.ProcessedCount, st.UserID)
	if err != nil {
		return fmt.Errorf("не удалось сохранить настройки: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateMainPrompt меняет основной системный промт пользователя.
func (s *Store) UpdateMainPrompt(ctx context.Context, userID, prompt string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE user_settings SET main_prompt = $1 WHERE uuid = $2`, prompt, userID)
	if err != nil {
		return fmt.Errorf("не удалось сохранить промт: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetInProgress сохраняет идентификатор текущего запроса в очереди.
//
// Значение служит признаком незавершённой генерации: интерфейс по нему
// понимает, что запрос ещё выполняется.
func (s *Store) SetInProgress(ctx context.Context, userID string, queryID any) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE user_settings SET inprogress = $1 WHERE uuid = $2`, queryID, userID)
	if err != nil {
		return fmt.Errorf("не удалось обновить состояние запроса: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PromptsForReview возвращает основной промт и промт выбранного пресета.
//
// Отсутствие пресета — не ошибка: раньше здесь возвращался pgx.ErrNoRows,
// из-за чего запрос отзыва падал, если пресет не был выбран.
func (s *Store) PromptsForReview(ctx context.Context, userID, presetID string) (mainPrompt, presetPrompt string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(main_prompt, '') FROM user_settings WHERE uuid = $1`, userID).Scan(&mainPrompt)
	if err != nil {
		return "", "", fmt.Errorf("не удалось получить основной промт: %w", err)
	}

	presetPrompt = ""
	if presetID == "" {
		return mainPrompt, presetPrompt, nil
	}

	var preset string
	err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(prompt, '') FROM custom_prompts WHERE uuid_uniq = $1 AND uuid_user = $2`,
		presetID, userID).Scan(&preset)
	if err != nil {
		if isNoRows(err) {
			// Пресета нет или он принадлежит другому пользователю —
			// в обоих случаях просто генерируем без него.
			return mainPrompt, "", nil
		}
		return "", "", fmt.Errorf("не удалось получить пресет: %w", err)
	}
	return mainPrompt, preset, nil
}

// CustomPrompt — сохранённый пользовательский шаблон запроса.
type CustomPrompt struct {
	ID     string `json:"uuid"`
	UserID string `json:"uuidUser"`
	Name   string `json:"name"`
	Prompt string `json:"promt"`
}

// AddCustomPrompt создаёт шаблон.
func (s *Store) AddCustomPrompt(ctx context.Context, userID, name, prompt string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO custom_prompts (uuid_user, name, prompt)
		VALUES ($1, $2, $3)
		RETURNING uuid_uniq::text
	`, userID, name, prompt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("не удалось сохранить шаблон: %w", err)
	}
	return id, nil
}

// UpdateCustomPrompt обновляет шаблон.
//
// Идентификатор шаблона всегда проверяется вместе с владельцем: без этой
// проверки можно было бы изменить чужой шаблон, зная его uuid.
func (s *Store) UpdateCustomPrompt(ctx context.Context, userID, promptID, name, prompt string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE custom_prompts
		SET name = $1, prompt = $2
		WHERE uuid_uniq = $3 AND uuid_user = $4
	`, name, prompt, promptID, userID)
	if err != nil {
		return fmt.Errorf("не удалось обновить шаблон: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListCustomPrompts возвращает шаблоны пользователя.
func (s *Store) ListCustomPrompts(ctx context.Context, userID string) ([]CustomPrompt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT uuid_uniq::text, uuid_user::text, name, COALESCE(prompt, '')
		FROM custom_prompts
		WHERE uuid_user = $1
		ORDER BY name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить шаблоны: %w", err)
	}
	defer rows.Close()

	out := make([]CustomPrompt, 0)
	for rows.Next() {
		var p CustomPrompt
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Prompt); err != nil {
			return nil, fmt.Errorf("ошибка чтения шаблона: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка перебора шаблонов: %w", err)
	}
	return out, nil
}

// DeleteCustomPrompt удаляет шаблон пользователя.
func (s *Store) DeleteCustomPrompt(ctx context.Context, userID, promptID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM custom_prompts WHERE uuid_uniq = $1 AND uuid_user = $2`, promptID, userID)
	if err != nil {
		return fmt.Errorf("не удалось удалить шаблон: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Memory — сохранённые факты о предпочтениях пользователя.
type Memory struct {
	UserID string `json:"uuid,omitempty"`
	Text   string `json:"memory"`
}

// CreateMemory создаёт пустую запись памяти.
func (s *Store) CreateMemory(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_memory (uuid, memory) VALUES ($1, '') ON CONFLICT (uuid) DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("не удалось создать запись памяти: %w", err)
	}
	return nil
}

// RememberMemory сохраняет память пользователя.
func (s *Store) RememberMemory(ctx context.Context, userID, text string) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO user_memory (uuid, memory) VALUES ($1, $2)
		ON CONFLICT (uuid) DO UPDATE SET memory = EXCLUDED.memory, updated_at = now()
	`, userID, text)
	if err != nil {
		return fmt.Errorf("не удалось сохранить память: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecallMemory возвращает память пользователя.
//
// Отсутствие записи — пустая память, а не ошибка: генерация должна работать и
// у нового пользователя.
func (s *Store) RecallMemory(ctx context.Context, userID string) (string, error) {
	var text string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(memory, '') FROM user_memory WHERE uuid = $1`, userID).Scan(&text)
	if err != nil {
		if isNoRows(err) {
			return "", nil
		}
		return "", fmt.Errorf("не удалось получить память: %w", err)
	}
	return text, nil
}
