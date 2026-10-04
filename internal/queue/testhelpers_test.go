package queue

import (
	"io"
	"log/slog"
)

// discardLogger возвращает логгер, отбрасывающий записи.
//
// Тесты не должны засорять вывод сообщениями о неудачных заданиях.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
