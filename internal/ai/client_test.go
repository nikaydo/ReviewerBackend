package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient создаёт клиента на поддельный сервер и отключает повторы.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := NewClient("test-key", srv.URL+"/api/v1", "https://example.test", "Test", 2*time.Second, 0)
	return c
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantID   string
		wantErr  bool
		wantReas bool
	}{
		{name: "пустое имя берёт модель по умолчанию", id: "", wantID: DefaultModel, wantReas: true},
		{name: "разрешённая модель", id: "google/gemini-2.5-flash", wantID: "google/gemini-2.5-flash", wantReas: true},
		{name: "модель без reasoning", id: "openai/gpt-4o-mini", wantID: "openai/gpt-4o-mini"},
		{name: "неизвестная модель", id: "mistralai/does-not-exist", wantErr: true},
		{name: "пустая строка с пробелами", id: "   ", wantID: DefaultModel, wantReas: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ResolveModel(tt.id)
			if tt.wantErr {
				var unknown *ErrUnknownModel
				if err == nil || !errors.As(err, &unknown) {
					t.Fatalf("ожидалась ErrUnknownModel, получено %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveModel вернул ошибку: %v", err)
			}
			if m.ID != tt.wantID {
				t.Errorf("ID = %q, ожидалось %q", m.ID, tt.wantID)
			}
			if m.Reasoning != tt.wantReas {
				t.Errorf("Reasoning = %v, ожидалось %v", m.Reasoning, tt.wantReas)
			}
		})
	}
}

func TestIsAllowed(t *testing.T) {
	if !IsAllowed("anthropic/claude-sonnet-4.6") {
		t.Error("разрешённая модель определена как запрещённая")
	}
	if IsAllowed("anthropic/claude-not-real") {
		t.Error("несуществующая модель прошла проверку")
	}
	if !IsAllowed("") {
		t.Error("пустое имя должно разрешаться в модель по умолчанию")
	}
}

func TestGenerateSendsExpectedRequest(t *testing.T) {
	var gotHeader http.Header
	var gotBody ChatCompletion

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("не удалось разобрать тело запроса: %v", err)
		}
		writeCompletion(w, "сгенерированный ответ", "рассуждение", "anthropic/claude-sonnet-4.6")
	})

	answer, err := client.Generate(context.Background(), Request{
		Model:            "anthropic/claude-sonnet-4.6",
		UserInput:        "напиши отзыв",
		System:           "ты редактор",
		AssistantHistory: "предыдущий ответ",
		ReasoningEffort:  "high",
	})
	if err != nil {
		t.Fatalf("Generate вернул ошибку: %v", err)
	}

	// Атрибуция обязательна для OpenRouter: без неё использование
	// не привязывается к приложению.
	if got := gotHeader.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q", got)
	}
	if got := gotHeader.Get("HTTP-Referer"); got != "https://example.test" {
		t.Errorf("HTTP-Referer = %q", got)
	}
	if got := gotHeader.Get("X-OpenRouter-Title"); got != "Test" {
		t.Errorf("X-OpenRouter-Title = %q", got)
	}

	if gotBody.Model != "anthropic/claude-sonnet-4.6" {
		t.Errorf("Model = %q", gotBody.Model)
	}
	if len(gotBody.Messages) != 3 {
		t.Fatalf("сообщений %d, ожидалось 3", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != RoleSystem || gotBody.Messages[0].Content != "ты редактор" {
		t.Errorf("первое сообщение = %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != RoleAssistant {
		t.Errorf("второе сообщение = %+v, ожидалась роль assistant", gotBody.Messages[1])
	}
	if gotBody.Messages[2].Role != RoleUser || gotBody.Messages[2].Content != "напиши отзыв" {
		t.Errorf("третье сообщение = %+v", gotBody.Messages[2])
	}
	if gotBody.Reasoning == nil || gotBody.Reasoning.Effort != "high" {
		t.Errorf("Reasoning = %+v, ожидался effort=high", gotBody.Reasoning)
	}

	if answer.Text != "сгенерированный ответ" {
		t.Errorf("Text = %q", answer.Text)
	}
	if answer.Reasoning != "рассуждение" {
		t.Errorf("Reasoning = %q", answer.Reasoning)
	}
	if answer.Model != "anthropic/claude-sonnet-4.6" {
		t.Errorf("Model = %q", answer.Model)
	}
}

func TestGenerateOmitsReasoningForPlainModels(t *testing.T) {
	var gotBody ChatCompletion

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeCompletion(w, "ответ", "", "openai/gpt-4o-mini")
	})

	_, err := client.Generate(context.Background(), Request{
		Model:     "openai/gpt-4o-mini",
		UserInput: "вопрос",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Модель без reasoning не должна получать поле reasoning: часть
	// провайдеров отклоняет запрос с неподдерживаемым параметром.
	if gotBody.Reasoning != nil {
		t.Errorf("Reasoning = %+v, ожидался nil", gotBody.Reasoning)
	}
}

func TestGenerateDefaultsReasoningEffort(t *testing.T) {
	var gotBody ChatCompletion

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeCompletion(w, "ответ", "", "google/gemini-2.5-flash")
	})

	_, err := client.Generate(context.Background(), Request{
		Model:     "google/gemini-2.5-flash",
		UserInput: "вопрос",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gotBody.Reasoning == nil || gotBody.Reasoning.Effort != "medium" {
		t.Errorf("Reasoning = %+v, ожидался effort=medium по умолчанию", gotBody.Reasoning)
	}
}

func TestGenerateRejectsUnknownModelBeforeRequest(t *testing.T) {
	var called atomic.Bool

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		writeCompletion(w, "ответ", "", "x")
	})

	_, err := client.Generate(context.Background(), Request{Model: "evil/model", UserInput: "x"})
	if err == nil {
		t.Fatal("неизвестная модель должна отклоняться")
	}
	if called.Load() {
		t.Error("запрос к провайдеру не должен отправляться для неизвестной модели")
	}
}

func TestGenerateHandlesHTTPStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{"лимит запросов", http.StatusTooManyRequests, `{"error":{"code":429,"message":"Rate limit exceeded"}}`, ErrRateLimited},
		{"нет доступа", http.StatusUnauthorized, `{"error":{"code":401,"message":"No auth credentials"}}`, ErrUnknownModelOrAPI},
		{"не оплачен", http.StatusPaymentRequired, `{"error":{"code":402,"message":"Insufficient credits"}}`, ErrUnknownModelOrAPI},
		{"ошибка провайдера", http.StatusBadGateway, `oops`, ErrUpstream},
		{"неизвестная ошибка", http.StatusBadRequest, `{"error":{"code":400,"message":"Bad model"}}`, ErrUnknownModelOrAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := client.Generate(context.Background(), Request{
				Model:     "anthropic/claude-sonnet-4.6",
				UserInput: "x",
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ошибка = %v, ожидалась %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateRetriesOnRateLimit(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":429,"message":"slow down"}}`))
			return
		}
		writeCompletion(w, "ответ", "", "anthropic/claude-sonnet-4.6")
	}))
	t.Cleanup(srv.Close)

	client := NewClient("k", srv.URL, "https://example.test", "Test", 2*time.Second, 2)
	// Задержка между попытками делается нулевой, чтобы тест не ждал.
	client.backoff = time.Millisecond

	answer, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if err != nil {
		t.Fatalf("Generate вернул ошибку: %v", err)
	}
	if answer.Text != "ответ" {
		t.Errorf("Text = %q", answer.Text)
	}
	if answer.Attempts != 2 {
		t.Errorf("Attempts = %d, ожидалось 2", answer.Attempts)
	}
}

func TestGenerateGivesUpAfterRetries(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"slow down"}}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("k", srv.URL, "https://example.test", "Test", 2*time.Second, 2)
	client.backoff = time.Millisecond

	_, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("ошибка = %v, ожидалась ErrRateLimited", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("попыток %d, ожидалось 3 (первая + два повтора)", got)
	}
}

func TestGenerateDoesNotRetryUnauthorized(t *testing.T) {
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"bad key"}}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient("k", srv.URL, "https://example.test", "Test", 2*time.Second, 3)
	client.backoff = time.Millisecond

	_, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	// Повторять запрос с неверным ключом бессмысленно: результат не изменится.
	if got := attempts.Load(); got != 1 {
		t.Errorf("попыток %d, ожидалась 1", got)
	}
}

func TestGenerateRejectsEmptyChoices(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	})

	_, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("ошибка = %v, ожидалась ErrEmptyResponse", err)
	}
}

func TestGenerateRejectsEmptyContent(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"   "}}]}`))
	})

	_, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("ошибка = %v, ожидалась ErrEmptyResponse", err)
	}
}

func TestGenerateHandlesNonJSONBody(t *testing.T) {
	// Балансировщик может вернуть HTML: такой ответ нельзя молча
	// разбирать как будто запрос удался.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>gateway</html>"))
	})

	_, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if err == nil {
		t.Fatal("ожидалась ошибка разбора не-JSON ответа")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("в тексте ошибки нет кода статуса: %v", err)
	}
}

func TestGenerateHonorsContextCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		writeCompletion(w, "ответ", "", "anthropic/claude-sonnet-4.6")
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})

	client := NewClient("k", srv.URL, "https://example.test", "Test", 30*time.Second, 0)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.Generate(ctx, Request{Model: "anthropic/claude-sonnet-4.6", UserInput: "x"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ошибка = %v, ожидался context.Canceled", err)
	}
}

func TestGenerateReportsUsage(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "anthropic/claude-sonnet-4.6",
			"choices": [{"message": {"content": "ответ"}}],
			"usage": {"prompt_tokens": 120, "completion_tokens": 340}
		}`))
	})

	answer, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if answer.PromptTokens != 120 || answer.CompletionTokens != 340 {
		t.Errorf("токены = %d/%d, ожидалось 120/340", answer.PromptTokens, answer.CompletionTokens)
	}
}

func TestGenerateTruncatesLongErrorDetail(t *testing.T) {
	long := strings.Repeat("A", 2000)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"` + long + `"}}`))
	})

	_, err := client.Generate(context.Background(), Request{
		Model:     "anthropic/claude-sonnet-4.6",
		UserInput: "x",
	})
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if len(err.Error()) > 700 {
		t.Errorf("текст ошибки слишком длинный: %d символов", len(err.Error()))
	}
}

// writeCompletion пишет успешный ответ OpenRouter.
func writeCompletion(w http.ResponseWriter, content, reasoning, model string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":    "gen-123",
		"model": model,
		"choices": []map[string]any{
			{
				"message": map[string]string{
					"content":   content,
					"reasoning": reasoning,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20},
	})
}
