package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nikaydo/reviewer/internal/database"
)

// problem — единый формат ошибки.
//
// В исходной версии в ответ писалась строка err.Error(), то есть наружу
// попадали тексты SQL-ошибок с названиями таблиц и колонок. Теперь наружу
// отдаётся короткое безопасное сообщение, а подробности уходят в лог.
type problem struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// writeJSON пишет ответ в JSON.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Заголовки и статус уже отправлены — изменить их нельзя,
		// остаётся только записать проблему в лог.
		slog.Default().Error("не удалось записать ответ", "err", err)
	}
}

// writeError пишет безопасную ошибку клиенту и подробности в лог.
func writeError(ctx context.Context, w http.ResponseWriter, log *slog.Logger, status int, code, public string, cause error) {
	if log == nil {
		log = slog.Default()
	}
	if cause != nil {
		log.LogAttrs(ctx, levelFor(status), "запрос не выполнен",
			slog.String("code", code),
			slog.String("public", public),
			slog.String("cause", cause.Error()),
		)
	}
	writeJSON(w, status, problem{Error: public, Code: code})
}

// levelFor выбирает уровень логирования по статусу ответа.
func levelFor(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status == http.StatusTooManyRequests, status == http.StatusUnauthorized, status == http.StatusForbidden:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// fail — короткая запись для ответов без причины.
func fail(ctx context.Context, w http.ResponseWriter, log *slog.Logger, status int, code, public string) {
	writeError(ctx, w, log, status, code, public, nil)
}

// failErr — запись с причиной, попадающей в лог.
func failErr(ctx context.Context, w http.ResponseWriter, log *slog.Logger, status int, code, public string, cause error) {
	writeError(ctx, w, log, status, code, public, cause)
}

// decodeJSON разбирает тело запроса с ограничением размера.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return fmt.Errorf("тело запроса превышает %d байт", maxBytes)
		}
		return fmt.Errorf("не удалось разобрать JSON: %w", err)
	}
	return nil
}

// notFoundOrInternal превращает ошибку доступа в понятный клиенту ответ.
//
// Ошибка «не найдено» возвращается как 404 и когда запись не существует,
// и когда она принадлежит другому пользователю: иначе по коду ответа можно
// перебором узнать о существовании чужих идентификаторов.
func notFoundOrInternal(ctx context.Context, w http.ResponseWriter, log *slog.Logger, err error) {
	if errors.Is(err, database.ErrNotFound) {
		fail(ctx, w, log, http.StatusNotFound, "not_found", "запись не найдена")
		return
	}
	failErr(ctx, w, log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
}
