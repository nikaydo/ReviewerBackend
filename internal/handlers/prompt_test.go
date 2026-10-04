package handlers

import (
	"strings"
	"testing"
)

func TestBuildPromptWithoutOptionalParts(t *testing.T) {
	got := BuildPrompt("основная инструкция", "", "")

	if got != "основная инструкция" {
		t.Errorf("BuildPrompt = %q, ожидалась только основная инструкция", got)
	}
}

func TestBuildPromptWithMemory(t *testing.T) {
	got := BuildPrompt("основная инструкция", "пользователь любит книги", "")

	if !strings.Contains(got, "основная инструкция") {
		t.Error("потеряна основная инструкция")
	}
	if !strings.Contains(got, "пользователь любит книги") {
		t.Error("память пользователя не попала в промт")
	}
}

func TestBuildPromptWithPreset(t *testing.T) {
	got := BuildPrompt("основная инструкция", "", "только сухие факты")

	if !strings.Contains(got, "только сухие факты") {
		t.Error("критерии пресета не попали в промт")
	}
}

func TestBuildPromptOrdersParts(t *testing.T) {
	// Порядок важен: память и критерии идут после основной инструкции,
	// иначе факты о пользователе вытесняют её из внимания модели.
	got := BuildPrompt("ОСНОВНАЯ", "ПАМЯТЬ", "ПРЕСЕТ")

	idxMain := strings.Index(got, "ОСНОВНАЯ")
	idxMemory := strings.Index(got, "ПАМЯТЬ")
	idxPreset := strings.Index(got, "ПРЕСЕТ")

	if idxMain < 0 || idxMemory < 0 || idxPreset < 0 {
		t.Fatalf("не все части попали в промт:\n%s", got)
	}
	if !(idxMain < idxMemory && idxMemory < idxPreset) {
		t.Errorf("неверный порядок частей (main=%d, memory=%d, preset=%d):\n%s",
			idxMain, idxMemory, idxPreset, got)
	}
}

func TestBuildPromptSeparatesParts(t *testing.T) {
	// Части должны разделяться переводом строки, иначе основная инструкция
	// и критерии склеятся в одно предложение и модель прочитает их как
	// один поток текста.
	got := BuildPrompt("основная", "память", "пресет")

	if strings.Contains(got, "основнаяпамять") {
		t.Error("части промта склеились без разделителя")
	}
}

func TestIsUUID(t *testing.T) {
	valid := []string{
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"123e4567-e89b-12d3-a456-426614174000",
	}
	for _, s := range valid {
		if !isUUID(s) {
			t.Errorf("isUUID(%q) = false, ожидалось true", s)
		}
	}

	invalid := []string{
		"",
		"не-uuid",
		"12345",
		"6ba7b810-9dad-11d1-80b4",
		"../../etc/passwd",
		"'; DROP TABLE users; --",
	}
	for _, s := range invalid {
		if isUUID(s) {
			t.Errorf("isUUID(%q) = true, ожидалось false", s)
		}
	}
}

func TestStringOr(t *testing.T) {
	if got := stringOr(true, "medium"); got != "medium" {
		t.Errorf("stringOr(true, medium) = %q", got)
	}
	// Для моделей без reasoning глубина размышления не передаётся.
	if got := stringOr(false, "medium"); got != "" {
		t.Errorf("stringOr(false, medium) = %q, ожидалась пустая строка", got)
	}
}

func TestSessionContextRoundTrip(t *testing.T) {
	// Идентификатор пользователя должен доезжать до обработчика через
	// контекст: именно так проверяется принадлежность ресурса запросу.
	// Иначе обработчик вынужден брать идентификатор из тела запроса,
	// а это и есть источник IDOR.
	ctx := withSession(t.Context(), session{
		UserID: "user-1",
		Login:  "nikaydo",
		Role:   "viewer",
	})

	got, ok := sessionFrom(ctx)
	if !ok {
		t.Fatal("сессия не извлеклась из контекста")
	}
	if got.UserID != "user-1" || got.Login != "nikaydo" || got.Role != "viewer" {
		t.Errorf("сессия = %+v", got)
	}
}

func TestSessionFromEmptyContext(t *testing.T) {
	if _, ok := sessionFrom(t.Context()); ok {
		t.Error("в пустом контексте не должно быть сессии")
	}
}
