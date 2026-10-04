package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/nikaydo/reviewer/internal/ai"
)

// settingsRequest — тело запроса на изменение настроек.
type settingsRequest struct {
	Request      string `json:"request"`
	Model        string `json:"model"`
	UseMemory    bool   `json:"memory"`
	MainPrompt   string `json:"mainPrompt"`
	MemoryPrompt string `json:"memoryPrompt"`
}

// memoryRequest — тело запроса на изменение памяти.
type memoryRequest struct {
	Text string `json:"text"`
}

// promptRequest — тело запроса для шаблонов.
type promptRequest struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// GetSettings возвращает настройки пользователя.
func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	settings, err := h.Store.SettingsByUser(ctx, s.UserID)
	if err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateSettings сохраняет настройки пользователя.
//
// Модель проверяется по белому списку: значение приходит из тела запроса и
// без проверки использовалось бы как идентификатор модели у провайдера.
func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req settingsRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}

	model, err := ai.ResolveModel(req.Model)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "unknown_model", "запрошена недоступная модель", err)
		return
	}

	current, err := h.Store.SettingsByUser(ctx, s.UserID)
	if err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}

	current.Request = req.Request
	current.Model = model.ID
	current.UseMemory = req.UseMemory
	if strings.TrimSpace(req.MainPrompt) != "" {
		current.MainPrompt = &req.MainPrompt
	}
	if strings.TrimSpace(req.MemoryPrompt) != "" {
		current.MemoryPrompt = &req.MemoryPrompt
	}

	if err := h.Store.UpdateSettings(ctx, current); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, current)
}

// GetMemory возвращает память пользователя.
func (h *Handlers) GetMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	text, err := h.Store.RecallMemory(ctx, s.UserID)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"memory": text})
}

// SetMemory записывает память пользователя.
func (h *Handlers) SetMemory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req memoryRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if len(req.Text) > maxPromptLength {
		fail(ctx, w, h.Log, http.StatusBadRequest, "memory_too_long", "слишком длинная запись")
		return
	}

	if err := h.Store.RememberMemory(ctx, s.UserID, req.Text); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListPrompts возвращает шаблоны пользователя.
func (h *Handlers) ListPrompts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	prompts, err := h.Store.ListCustomPrompts(ctx, s.UserID)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}
	writeJSON(w, http.StatusOK, prompts)
}

// CreatePrompt создаёт шаблон.
func (h *Handlers) CreatePrompt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req promptRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(ctx, w, h.Log, http.StatusBadRequest, "empty_name", "укажите название шаблона")
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		fail(ctx, w, h.Log, http.StatusBadRequest, "empty_prompt", "текст шаблона не может быть пустым")
		return
	}

	id, err := h.Store.AddCustomPrompt(ctx, s.UserID, req.Name, req.Prompt)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"uuid": id})
}

// UpdatePrompt обновляет шаблон.
//
// Идентификатор шаблона всегда сверяется с владельцем внутри запроса к базе.
// Раньше обработчик удаления передавал в базу только идентификатор из формы,
// из-за чего любой авторизованный пользователь мог удалить чужой шаблон.
func (h *Handlers) UpdatePrompt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	promptID := r.PathValue("promptID")
	if !isUUID(promptID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор шаблона")
		return
	}

	var req promptRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Prompt) == "" {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_prompt", "название и текст шаблона обязательны")
		return
	}

	if err := h.Store.UpdateCustomPrompt(ctx, s.UserID, promptID, strings.TrimSpace(req.Name), req.Prompt); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeletePrompt удаляет шаблон пользователя.
func (h *Handlers) DeletePrompt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	promptID := r.PathValue("promptID")
	if !isUUID(promptID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор шаблона")
		return
	}

	if err := h.Store.DeleteCustomPrompt(ctx, s.UserID, promptID); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// isUUID проверяет, что строка является корректным UUID.
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
