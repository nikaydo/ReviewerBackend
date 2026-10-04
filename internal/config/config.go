// Package config загружает и проверяет конфигурацию приложения.
//
// Значения читаются из системного окружения, с возможностью подгрузки из
// файла .env. Конфигурация валидируется при старте: если обязательное
// поле не задано или осталось с заглушкой, приложение не запускается.
// Так дефект конфигурации обнаруживается сразу, а не при первом запросе.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Значение-заглушка, которым помечены секреты в .env.example.
const placeholder = "CHANGE_ME"

// Config содержит всю конфигурацию приложения.
type Config struct {
	// HTTP-сервер.
	Host string `env:"HOST"                 envDefault:"0.0.0.0"`
	Port int    `env:"PORT"                 envDefault:"8080"`

	// Хранилище.
	DatabaseURL   string `env:"DATABASE_URL,required"`
	MigrationsDir string `env:"MIGRATIONS_DIR" envDefault:"db/migrations"`

	// JWT. Access-токен живёт минуты, refresh — часы.
	JWTSecret        string        `env:"JWT_SECRET,required"`
	JWTRefreshSecret string        `env:"JWT_REFRESH_SECRET,required"`
	AccessTokenTTL   time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"60m"`
	RefreshTokenTTL  time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"720h"`
	Issuer           string        `env:"JWT_ISSUER"       envDefault:"reviewer"`

	// Cookie с access-токеном.
	CookieName   string        `env:"COOKIE_NAME"   envDefault:"jwt"`
	CookieTTL    time.Duration `env:"COOKIE_TTL"    envDefault:"24h"`
	CookieSecure bool          `env:"COOKIE_SECURE" envDefault:"true"`

	// Очередь генерации.
	QueuePollInterval    time.Duration `env:"QUEUE_POLL_INTERVAL" envDefault:"500ms"`
	QueueBetweenRequests time.Duration `env:"QUEUE_BETWEEN_REQUESTS" envDefault:"1s"`
	QueueConcurrency     int           `env:"QUEUE_CONCURRENCY"     envDefault:"2"`

	// Ограничение частоты запросов на эндпоинты авторизации.
	AuthRateLimit   int           `env:"AUTH_RATE_LIMIT"   envDefault:"5"`
	AuthRateWindow  time.Duration `env:"AUTH_RATE_WINDOW"  envDefault:"1m"`
	MaxRequestBytes int64         `env:"MAX_REQUEST_BYTES" envDefault:"1048576"`

	// OpenRouter.
	OpenRouterAPIKey  string        `env:"OPENROUTER_API_KEY,required"`
	OpenRouterBaseURL string        `env:"OPENROUTER_BASE_URL"    envDefault:"https://openrouter.ai/api/v1"`
	OpenRouterReferer string        `env:"OPENROUTER_REFERER"     envDefault:"https://github.com/nikaydo/ReviewerBackend"`
	OpenRouterTitle   string        `env:"OPENROUTER_TITLE"       envDefault:"Reviewer"`
	OpenRouterTimeout time.Duration `env:"OPENROUTER_TIMEOUT"     envDefault:"120s"`
	OpenRouterRetries int           `env:"OPENROUTER_RETRIES"     envDefault:"2"`

	// Промты по умолчанию.
	DefaultMainPrompt  string `env:"DEFAULT_MAIN_PROMPT,required"`
	MemorizationPrompt string `env:"MEMORIZATION_PROMPT,required"`
}

// Addr возвращает адрес HTTP-сервера в формате host:port.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Load читает конфигурацию из .env (если файл есть) и из переменных окружения,
// после чего валидирует её.
//
// Порядок приоритета: переменные окружения перекрывают значения из .env,
// поэтому в продакшене файл .env можно не монтировать целиком.
func Load() (Config, error) {
	// Отсутствие .env не является ошибкой: в Docker и CI конфигурация
	// приходит исключительно через переменные окружения.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Stat(".env"); statErr == nil {
			return Config{}, fmt.Errorf("не удалось прочитать .env: %w", err)
		}
	}

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("не удалось разобрать конфигурацию: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate проверяет значения, которые env-парсер не может проверить сам.
func (c Config) Validate() error {
	var problems []string

	problems = appendSecretProblems(problems, "JWT_SECRET", c.JWTSecret)
	problems = appendSecretProblems(problems, "JWT_REFRESH_SECRET", c.JWTRefreshSecret)
	problems = appendSecretProblems(problems, "OPENROUTER_API_KEY", c.OpenRouterAPIKey)

	if c.JWTSecret != "" && c.JWTSecret == c.JWTRefreshSecret {
		problems = append(problems, "JWT_SECRET и JWT_REFRESH_SECRET не должны совпадать")
	}
	if len(c.JWTSecret) > 0 && len(c.JWTSecret) < 32 {
		problems = append(problems, fmt.Sprintf(
			"JWT_SECRET слишком короткий (%d символов), нужно минимум 32: сгенерируйте через openssl rand -base64 48",
			len(c.JWTSecret)))
	}
	if len(c.JWTRefreshSecret) > 0 && len(c.JWTRefreshSecret) < 32 {
		problems = append(problems, fmt.Sprintf(
			"JWT_REFRESH_SECRET слишком короткий (%d символов), нужно минимум 32", len(c.JWTRefreshSecret)))
	}
	if c.AccessTokenTTL <= 0 {
		problems = append(problems, "ACCESS_TOKEN_TTL должен быть больше нуля")
	}
	if c.RefreshTokenTTL <= c.AccessTokenTTL {
		problems = append(problems, "REFRESH_TOKEN_TTL должен быть больше ACCESS_TOKEN_TTL")
	}
	if c.Port < 1 || c.Port > 65535 {
		problems = append(problems, fmt.Sprintf("PORT=%d вне диапазона 1-65535", c.Port))
	}
	if c.QueueConcurrency < 1 {
		problems = append(problems, "QUEUE_CONCURRENCY должен быть не меньше 1")
	}
	if c.AuthRateLimit < 1 {
		problems = append(problems, "AUTH_RATE_LIMIT должен быть не меньше 1")
	}
	if c.MaxRequestBytes < 1024 {
		problems = append(problems, "MAX_REQUEST_BYTES должен быть не меньше 1024")
	}
	if err := validateDatabaseURL(c.DatabaseURL); err != nil {
		problems = append(problems, err.Error())
	}
	if strings.TrimSpace(c.DefaultMainPrompt) == "" {
		problems = append(problems, "DEFAULT_MAIN_PROMPT не может быть пустым")
	}
	if strings.TrimSpace(c.MemorizationPrompt) == "" {
		problems = append(problems, "MEMORIZATION_PROMPT не может быть пустым")
	}

	if len(problems) > 0 {
		return fmt.Errorf("некорректная конфигурация:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// appendSecretProblems добавляет в список проблемы по «секретным» полям.
func appendSecretProblems(problems []string, name, value string) []string {
	switch {
	case strings.TrimSpace(value) == "":
		problems = append(problems, fmt.Sprintf("%s не задан", name))
	case value == placeholder:
		problems = append(problems, fmt.Sprintf(
			"%s остался со значением-заглушкой %s из .env.example — задайте настоящее значение", name, placeholder))
	case strings.HasPrefix(value, "postgres://your") || strings.Contains(value, "USER:PASSWORD@"):
		problems = append(problems, fmt.Sprintf("%s выглядит как шаблон из .env.example — подставьте реальные данные", name))
	}
	return problems
}

// validateDatabaseURL проверяет, что строка подключения разбирается и указывает
// на PostgreSQL.
func validateDatabaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("DATABASE_URL не задан")
	}
	if raw == placeholder || strings.Contains(raw, "USER:PASSWORD@") {
		return fmt.Errorf(
			"DATABASE_URL остался шаблоном из .env.example — подставьте реальные данные")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("DATABASE_URL не разбирается: %w", err)
	}
	switch u.Scheme {
	case "postgres", "postgresql":
	default:
		return fmt.Errorf("DATABASE_URL должен использовать схему postgres:// или postgresql://, получено %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("DATABASE_URL не содержит хоста")
	}
	return nil
}
