package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/nikaydo/reviewer/internal/ai"
	"github.com/nikaydo/reviewer/internal/queue"
)

// createReviewRequest — тело запроса на генерацию отзыва.
type createReviewRequest struct {
	Product string `json:"product"`
	Preset  string `json:"preset"`
	Model   string `json:"model"`
}

// updateReviewRequest — тело запроса на редактирование отзыва.
type updateReviewRequest struct {
	ReviewID string `json:"uuid"`
	Answer   string `json:"answer"`
}

// favoriteRequest — тело запроса на переключение избранного.
type favoriteRequest struct {
	ReviewID string `json:"uuid"`
	Favorite bool   `json:"favorite"`
}

// createTitleRequest — тело запроса на генерацию заголовка.
type createTitleRequest struct {
	ReviewID string `json:"uuid"`
	Request  string `json:"request"`
}

// updateTitleRequest — тело запроса на редактирование заголовка.
type updateTitleRequest struct {
	ReviewID string `json:"uuid"`
	Title    string `json:"title"`
}

// maxPromptLength ограничивает длину текста, отправляемого модели.
const maxPromptLength = 8000

// ListReviews возвращает отзывы пользователя.
func (h *Handlers) ListReviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	reviews, err := h.Store.ListReviews(ctx, s.UserID)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

// CreateReview ставит генерацию отзыва в очередь.
func (h *Handlers) CreateReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req createReviewRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}

	product := strings.TrimSpace(req.Product)
	if product == "" {
		fail(ctx, w, h.Log, http.StatusBadRequest, "empty_request", "укажите, для чего сгенерировать отзыв")
		return
	}
	if len(product) > maxPromptLength {
		fail(ctx, w, h.Log, http.StatusBadRequest, "request_too_long", "слишком длинный запрос")
		return
	}

	// Модель проверяется по белому списку: раньше её идентификатор брался
	// из формы без проверки, и любой пользователь мог запустить любую
	// модель на чужие деньги.
	model, err := ai.ResolveModel(req.Model)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "unknown_model", "запрошена недоступная модель", err)
		return
	}

	settings, err := h.Store.SettingsByUser(ctx, s.UserID)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}

	mainPrompt, presetPrompt, err := h.Store.PromptsForReview(ctx, s.UserID, req.Preset)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}

	memory := ""
	if settings.UseMemory {
		// Память подтягивается только когда включена в настройках.
		// В исходной версии память вычислялась в переменную syst, а
		// следующей строкой затиралась, то есть на промт не влияла никогда.
		memory, err = h.Store.RecallMemory(ctx, s.UserID)
		if err != nil {
			failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
			return
		}
	}

	jobID, err := h.Queue.Submit(ctx, queue.Job{
		ID:              uuid.New(),
		UserID:          s.UserID,
		Target:          queue.TargetReview,
		Model:           model.ID,
		Request:         product,
		SystemPrompt:    BuildPrompt(mainPrompt, memory, presetPrompt),
		ReasoningEffort: stringOr(model.Reasoning, "medium"),
		UseMemory:       settings.UseMemory,
	})
	if err != nil {
		if errors.Is(err, queue.ErrQueueClosed) {
			fail(ctx, w, h.Log, http.StatusServiceUnavailable, "queue_closed", "сервис завершает работу")
			return
		}
		failErr(ctx, w, h.Log, http.StatusServiceUnavailable, "queue_full", "очередь перегружена, попробуйте позже", err)
		return
	}

	if err := h.Store.SetInProgress(ctx, s.UserID, jobID.String()); err != nil {
		h.Log.Error("не удалось сохранить состояние запроса", "job", jobID, "err", err)
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"uuid": jobID.String()})
}

// UpdateReview перезаписывает текст отзыва.
func (h *Handlers) UpdateReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req updateReviewRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if !isUUID(req.ReviewID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор отзыва")
		return
	}

	// Идентификатор владельца берётся из сессии, а не из тела запроса:
	// иначе можно было бы править чужие отзывы.
	if err := h.Store.UpdateReviewAnswer(ctx, s.UserID, req.ReviewID, req.Answer); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteReview удаляет отзыв.
func (h *Handlers) DeleteReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	reviewID := r.PathValue("reviewID")
	if !isUUID(reviewID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор отзыва")
		return
	}

	if err := h.Store.DeleteReview(ctx, s.UserID, reviewID); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Favorite переключает отметку «избранное».
func (h *Handlers) Favorite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req favoriteRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if !isUUID(req.ReviewID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор отзыва")
		return
	}

	if err := h.Store.SetReviewFavorite(ctx, s.UserID, req.ReviewID, req.Favorite); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateTitle ставит генерацию заголовка в очередь.
func (h *Handlers) CreateTitle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req createTitleRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if !isUUID(req.ReviewID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор отзыва")
		return
	}
	if strings.TrimSpace(req.Request) == "" {
		fail(ctx, w, h.Log, http.StatusBadRequest, "empty_request", "укажите текст для заголовка")
		return
	}

	// Отзыв читается с проверкой владельца: заголовок нельзя привязать
	// к чужому отзыву.
	review, err := h.Store.ReviewByID(ctx, s.UserID, req.ReviewID)
	if err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}

	settings, err := h.Store.SettingsByUser(ctx, s.UserID)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}

	model, err := ai.ResolveModel(settings.Model)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "unknown_model", "в настройках указана недоступная модель", err)
		return
	}

	jobID, err := h.Queue.Submit(ctx, queue.Job{
		ID:               uuid.New(),
		UserID:           s.UserID,
		ReviewID:         review.ID,
		Target:           queue.TargetTitle,
		Model:            model.ID,
		Request:          strings.TrimSpace(req.Request),
		AssistantHistory: review.Answer,
		ReasoningEffort:  stringOr(model.Reasoning, "low"),
	})
	if err != nil {
		if errors.Is(err, queue.ErrQueueClosed) {
			fail(ctx, w, h.Log, http.StatusServiceUnavailable, "queue_closed", "сервис завершает работу")
			return
		}
		failErr(ctx, w, h.Log, http.StatusServiceUnavailable, "queue_full", "очередь перегружена, попробуйте позже", err)
		return
	}

	if err := h.Store.SetInProgress(ctx, s.UserID, jobID.String()); err != nil {
		h.Log.Error("не удалось сохранить состояние запроса", "job", jobID, "err", err)
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"uuid": jobID.String()})
}

// UpdateTitle меняет заголовок на заданный пользователем.
func (h *Handlers) UpdateTitle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var req updateTitleRequest
	if err := decodeJSON(w, r, &req, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if !isUUID(req.ReviewID) {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор отзыва")
		return
	}

	if err := h.Store.UpdateReviewTitle(ctx, s.UserID, req.ReviewID, req.Title); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateMainPrompt меняет основной системный промт пользователя.
func (h *Handlers) UpdateMainPrompt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := decodeJSON(w, r, &body, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		fail(ctx, w, h.Log, http.StatusBadRequest, "empty_prompt", "промт не может быть пустым")
		return
	}
	if len(body.Prompt) > maxPromptLength {
		fail(ctx, w, h.Log, http.StatusBadRequest, "prompt_too_long", "слишком длинный промт")
		return
	}

	if err := h.Store.UpdateMainPrompt(ctx, s.UserID, body.Prompt); err != nil {
		notFoundOrInternal(ctx, w, h.Log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// QueueStatus возвращает положение задания в очереди.
func (h *Handlers) QueueStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if _, ok := sessionFrom(ctx); !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}

	raw := r.PathValue("jobID")
	id, err := uuid.Parse(raw)
	if err != nil {
		fail(ctx, w, h.Log, http.StatusBadRequest, "invalid_uuid", "некорректный идентификатор задания")
		return
	}

	if jobErr := h.Queue.ErrFor(id); jobErr != nil {
		// Причина ошибки скрыта от клиента: текст ошибки провайдера может
		// содержать детали, не предназначенные для показа пользователю.
		h.Log.Info("задание завершилось с ошибкой", "job", id, "err", jobErr)
		writeJSON(w, http.StatusOK, map[string]any{
			"Position": 0,
			"Total":    0,
			"Failed":   true,
		})
		return
	}

	writeJSON(w, http.StatusOK, h.Queue.Position(id))
}

// BuildPrompt собирает итоговый системный промт.
//
// Порядок частей важен: инструкции модели должны идти после фактов о
// пользователе, иначе факты вытесняют инструкции из внимания модели.
func BuildPrompt(mainPrompt, memory, preset string) string {
	var b strings.Builder
	b.WriteString(mainPrompt)

	if memory != "" {
		b.WriteString("\n\nУчти следующую информацию о пользователе, чтобы отзыв лучше ")
		b.WriteString("соответствовал его запросу. Используй только то, что относится к теме; ")
		b.WriteString("остальное проигнорируй.\n")
		b.WriteString(memory)
	}

	if preset != "" {
		b.WriteString("\n\nОбязательно приведи ответ в соответствие с этими критериями:\n")
		b.WriteString(preset)
	}

	return b.String()
}

// MemorizationPrompt возвращает шаблон промта для памяти пользователя.
func (h *Handlers) MemorizationPrompt(ctx context.Context, userID string) (string, error) {
	settings, err := h.Store.SettingsByUser(ctx, userID)
	if err != nil {
		return "", err
	}
	if settings.MemoryPrompt != nil && strings.TrimSpace(*settings.MemoryPrompt) != "" {
		return *settings.MemoryPrompt, nil
	}
	return h.Cfg.MemorizationPrompt, nil
}

// stringOr возвращает значение, если флаг включён, иначе запасное.
func stringOr(enabled bool, value string) string {
	if enabled {
		return value
	}
	return ""
}
