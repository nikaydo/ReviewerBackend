package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// baseEnv возвращает набор переменных, проходящих валидацию.
func baseEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":        "postgres://user:pass@localhost:5432/db?sslmode=disable",
		"JWT_SECRET":          strings.Repeat("a", 48),
		"JWT_REFRESH_SECRET":  strings.Repeat("b", 48),
		"OPENROUTER_API_KEY":  "sk-or-v1-test-key-value",
		"DEFAULT_MAIN_PROMPT": "system prompt",
		"MEMORIZATION_PROMPT": "memorization prompt",
	}
}

// setEnv выставляет переменные окружения на время теста.
func setEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for k, v := range values {
		t.Setenv(k, v)
	}
	// Гарантируем, что файл .env не повлияет на тест.
	if wd, err := os.Getwd(); err == nil {
		prev, had := os.LookupEnv("PWD")
		_ = os.Chdir(t.TempDir())
		t.Cleanup(func() {
			_ = os.Chdir(wd)
			if had {
				os.Setenv("PWD", prev)
			}
		})
	}
}

func TestLoadAcceptsValidConfig(t *testing.T) {
	setEnv(t, baseEnv())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load вернул ошибку: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, ожидалось 8080", cfg.Port)
	}
	if cfg.AccessTokenTTL != time.Hour {
		t.Errorf("AccessTokenTTL = %v, ожидалось 1h", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 30*24*time.Hour {
		t.Errorf("RefreshTokenTTL = %v, ожидалось 720h", cfg.RefreshTokenTTL)
	}
	if cfg.QueueConcurrency != 2 {
		t.Errorf("QueueConcurrency = %d, ожидалось 2", cfg.QueueConcurrency)
	}
	if cfg.Addr() != "0.0.0.0:8080" {
		t.Errorf("Addr = %q", cfg.Addr())
	}
}

func TestValidateRejectsPlaceholders(t *testing.T) {
	// Каждое из этих значений — типичная ошибка: файл .env.example скопирован,
	// но не заполнен. Приложение обязано падать на старте, а не работать
	// с ключом «CHANGE_ME».
	fields := map[string]string{
		"JWT_SECRET":         "JWT_SECRET",
		"JWT_REFRESH_SECRET": "JWT_REFRESH_SECRET",
		"OPENROUTER_API_KEY": "OPENROUTER_API_KEY",
	}

	for field, label := range fields {
		t.Run(field, func(t *testing.T) {
			values := baseEnv()
			values[field] = "CHANGE_ME"
			setEnv(t, values)

			_, err := Load()
			if err == nil {
				t.Fatalf("%s остался заглушкой, но приложение запустилось", label)
			}
			if !strings.Contains(err.Error(), label) {
				t.Errorf("ошибка не упоминает поле %s: %v", label, err)
			}
		})
	}
}

func TestValidateRejectsTemplateDatabaseURL(t *testing.T) {
	values := baseEnv()
	values["DATABASE_URL"] = "postgres://USER:PASSWORD@localhost:5432/db"
	setEnv(t, values)

	if _, err := Load(); err == nil {
		t.Fatal("шаблон в DATABASE_URL не должен приниматься")
	}
}

func TestValidateRejectsShortSecret(t *testing.T) {
	values := baseEnv()
	values["JWT_SECRET"] = "short"
	setEnv(t, values)

	_, err := Load()
	if err == nil {
		t.Fatal("короткий секрет не должен приниматься")
	}
	if !strings.Contains(err.Error(), "минимум 32") {
		t.Errorf("ошибка не объясняет требование к длине: %v", err)
	}
}

func TestValidateRejectsIdenticalSecrets(t *testing.T) {
	values := baseEnv()
	values["JWT_SECRET"] = strings.Repeat("x", 48)
	values["JWT_REFRESH_SECRET"] = strings.Repeat("x", 48)
	setEnv(t, values)

	_, err := Load()
	if err == nil {
		t.Fatal("одинаковые секреты не должны приниматься")
	}
	if !strings.Contains(err.Error(), "не должны совпадать") {
		t.Errorf("неожиданный текст ошибки: %v", err)
	}
}

func TestValidateRejectsNonPostgresURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"mysql", "mysql://user:pass@localhost:3306/db"},
		{"пустая строка", ""},
		{"без схемы", "localhost:5432"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := baseEnv()
			values["DATABASE_URL"] = tt.url
			setEnv(t, values)

			if _, err := Load(); err == nil {
				t.Fatalf("DATABASE_URL=%q не должен приниматься", tt.url)
			}
		})
	}
}

func TestValidateRejectsRefreshTTLShorterThanAccess(t *testing.T) {
	values := baseEnv()
	values["ACCESS_TOKEN_TTL"] = "2h"
	values["REFRESH_TOKEN_TTL"] = "1h"
	setEnv(t, values)

	_, err := Load()
	if err == nil {
		t.Fatal("refresh-токен должен жить дольше access-токена")
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"0", "70000", "-1"} {
		t.Run(port, func(t *testing.T) {
			values := baseEnv()
			values["PORT"] = port
			setEnv(t, values)

			if _, err := Load(); err == nil {
				t.Fatalf("порт %s не должен приниматься", port)
			}
		})
	}
}

func TestValidateRejectsZeroConcurrency(t *testing.T) {
	values := baseEnv()
	values["QUEUE_CONCURRENCY"] = "0"
	setEnv(t, values)

	if _, err := Load(); err == nil {
		t.Fatal("нулевая параллельность очереди не должна приниматься")
	}
}

func TestValidateRejectsEmptyPrompts(t *testing.T) {
	values := baseEnv()
	values["DEFAULT_MAIN_PROMPT"] = "   "
	setEnv(t, values)

	if _, err := Load(); err == nil {
		t.Fatal("пустой основной промт не должен приниматься")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	values := baseEnv()
	delete(values, "DATABASE_URL")
	setEnv(t, values)

	if _, err := Load(); err == nil {
		t.Fatal("отсутствие DATABASE_URL должно приводить к ошибке")
	}
}
