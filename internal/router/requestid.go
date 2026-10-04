package router

import (
	"context"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// requestIDKey — тип ключа для идентификатора запроса в контексте.
// Тип приватный, чтобы случайно не пересечься с чужим ключом.
type requestIDKey struct{}

// WithRequestID кладёт идентификатор запроса в контекст.
//
// Идентификатор нужен, чтобы связать записи журнала одного запроса:
// без него в логах невозможно понять, к какому обращению относится
// сообщение об ошибке.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// requestIDFrom возвращает идентификатор запроса из контекста.
func requestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return chimw.GetReqID(ctx)
}
