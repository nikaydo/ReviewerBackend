// Package ai реализует обращение к OpenRouter — единому шлюзу к разным
// LLM-провайдерам.
//
// Отличия от прямого вызова API одного провайдера, которые здесь учтены:
//
//   - OpenRouter нормализует режим размышления: у моделей с reasoning
//     текст рассуждений приходит в отдельном поле reasoning. Раньше он
//     вырезался из ответа вручную по маркеру </think>, что приводило к
//     панике, если модель не использовала этот маркер.
//   - Ответ может прийти с кодом 4xx или 5xx и содержать описание ошибки
//     в поле error. Раньше тело ответа разбиралось как будто запрос удался.
//   - Есть ограничение частоты запросов, поэтому 429 обрабатывается
//     повторами с задержкой, а не молча теряется.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// maxResponseBytes ограничивает размер читаемого тела ответа.
const maxResponseBytes = 4 << 20 // 4 МиБ

// Ошибки, которые вызывающая сторона может классифицировать.
var (
	// ErrUnknownModelOrAPI — модель отклонена шлюзом или неизвестна.
	ErrUnknownModelOrAPI = errors.New("запрос отклонён шлюзом")
	// ErrRateLimited — превышен лимит запросов, повторы не помогли.
	ErrRateLimited = errors.New("превышен лимит запросов к API")
	// ErrUpstream — временная ошибка на стороне шлюза или провайдера.
	ErrUpstream = errors.New("ошибка на стороне API")
	// ErrEmptyResponse — модель не вернула ни одного ответа.
	ErrEmptyResponse = errors.New("модель вернула пустой ответ")
)

// Message — одно сообщение в диалоге с моделью.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Роли сообщений.
const (
	RoleSystem    = "system"
	RoleAssistant = "assistant"
	RoleUser      = "user"
)

// Request — запрос на генерацию.
type Request struct {
	Model     string
	UserInput string
	System    string
	// AssistantHistory — предыдущие ответы модели, добавляются перед
	// пользовательским вводом.
	AssistantHistory string
	ReasoningEffort  string
}

// Answer — результат генерации.
type Answer struct {
	Text string
	// Reasoning — текст рассуждений модели, если она его вернула.
	Reasoning string
	// Model — фактически использованная модель.
	Model string
	// PromptTokens и CompletionTokens — расход токенов по ответу шлюза.
	PromptTokens     int
	CompletionTokens int
	// Attempts — сколько запросов ушло с учётом повторов.
	Attempts int
}

// ChatCompletion — формат запроса OpenRouter.
type ChatCompletion struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	// Reasoning включает режим размышления и задаёт его глубину.
	Reasoning *Reasoning `json:"reasoning,omitempty"`
}

// Reasoning управляет режимом размышления.
type Reasoning struct {
	Effort string `json:"effort,omitempty"`
}

// chatCompletionResponse — формат ответа OpenRouter.
type chatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	// Error заполняется при неуспешном ответе.
	Error *apiError `json:"error"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (e *apiError) String() string {
	if e == nil {
		return ""
	}
	if e.Code != 0 {
		return fmt.Sprintf("%d %s", e.Code, e.Message)
	}
	return e.Message
}

// Client обращается к OpenRouter.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	referer    string
	title      string
	retries    int
	backoff    time.Duration
	sleep      func(ctx context.Context, d time.Duration) error
}

// NewClient создаёт клиента OpenRouter.
func NewClient(apiKey, baseURL, referer, title string, timeout time.Duration, retries int) *Client {
	if retries < 0 {
		retries = 0
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{
		// Таймаут обязателен: без него запрос к LLM может висеть бесконечно
		// и держать соединение из пула.
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		referer:    referer,
		title:      title,
		retries:    retries,
		backoff:    500 * time.Millisecond,
		sleep:      sleepCtx,
	}
}

// Generate выполняет запрос к модели и возвращает ответ.
func (c *Client) Generate(ctx context.Context, req Request) (Answer, error) {
	model, err := ResolveModel(req.Model)
	if err != nil {
		return Answer{}, err
	}

	var messages []Message
	if req.System != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: req.System})
	}
	if req.AssistantHistory != "" {
		messages = append(messages, Message{Role: RoleAssistant, Content: req.AssistantHistory})
	}
	messages = append(messages, Message{Role: RoleUser, Content: req.UserInput})

	payload := ChatCompletion{
		Model:    model.ID,
		Messages: messages,
	}
	if model.Reasoning {
		effort := req.ReasoningEffort
		if effort == "" {
			effort = "medium"
		}
		payload.Reasoning = &Reasoning{Effort: effort}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Answer{}, fmt.Errorf("не удалось собрать запрос: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			// Задержка растёт экспоненциально: 0.5с, 1с, 2с…
			delay := time.Duration(float64(c.backoff) * math.Pow(2, float64(attempt-1)))
			if err := c.sleep(ctx, delay); err != nil {
				return Answer{}, err
			}
		}

		resp, err := c.do(ctx, body, model.ID)
		if err != nil {
			lastErr = err
			if isRetryable(err) {
				continue
			}
			return Answer{}, err
		}
		resp.Attempts = attempt + 1
		return resp, nil
	}

	if lastErr == nil {
		lastErr = ErrUpstream
	}
	return Answer{}, fmt.Errorf("после %d попыток: %w", c.retries+1, lastErr)
}

// do выполняет один HTTP-запрос и разбирает ответ.
func (c *Client) do(ctx context.Context, body []byte, modelID string) (Answer, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Answer{}, fmt.Errorf("не удалось создать запрос: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	// Заголовки атрибуции: без них OpenRouter не привязывает использование
	// к приложению и не показывает его в рейтингах.
	httpReq.Header.Set("HTTP-Referer", c.referer)
	httpReq.Header.Set("X-OpenRouter-Title", c.title)
	httpReq.Header.Set("X-OpenRouter-Categories", "writing-assistant")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Answer{}, fmt.Errorf("запрос к OpenRouter не выполнен: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		_ = httpResp.Body.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes))
	if err != nil {
		return Answer{}, fmt.Errorf("не удалось прочитать ответ: %w", err)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// Тело может оказаться не JSON (например, HTML от балансировщика) —
		// тогда возвращаем код статуса, чтобы вызывающий знал причину.
		return Answer{}, statusError(httpResp.StatusCode, httpResp.Status, string(raw))
	}

	if parsed.Error != nil {
		return Answer{}, statusError(httpResp.StatusCode, parsed.Error.String(), "")
	}
	if httpResp.StatusCode != http.StatusOK {
		return Answer{}, statusError(httpResp.StatusCode, httpResp.Status, "")
	}
	if len(parsed.Choices) == 0 {
		return Answer{}, ErrEmptyResponse
	}

	choice := parsed.Choices[0]
	text := strings.TrimSpace(choice.Message.Content)
	if text == "" {
		return Answer{}, ErrEmptyResponse
	}

	usedModel := parsed.Model
	if usedModel == "" {
		usedModel = modelID
	}
	return Answer{
		Text:             text,
		Reasoning:        strings.TrimSpace(choice.Message.Reasoning),
		Model:            usedModel,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
	}, nil
}

// statusError превращает ошибочный ответ шлюза в типизированную ошибку.
//
// Код статуса включается в текст всегда: тело ответа может оказаться
// HTML-страницей балансировщика, и без кода ошибку не разобрать.
func statusError(code int, status, detail string) error {
	if detail == "" {
		detail = status
	}
	if len(detail) > 500 {
		detail = detail[:500] + "…"
	}
	prefix := fmt.Sprintf("HTTP %d", code)
	switch {
	case code == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s: %s", ErrRateLimited, prefix, detail)
	case code == http.StatusUnauthorized || code == http.StatusForbidden || code == http.StatusPaymentRequired:
		return fmt.Errorf("%w: %s: %s", ErrUnknownModelOrAPI, prefix, detail)
	case code >= 500:
		return fmt.Errorf("%w: %s: %s", ErrUpstream, prefix, detail)
	default:
		return fmt.Errorf("%w: %s: %s", ErrUnknownModelOrAPI, prefix, detail)
	}
}

// isRetryable сообщает, стоит ли повторить запрос.
func isRetryable(err error) bool {
	return errors.Is(err, ErrRateLimited) || errors.Is(err, ErrUpstream)
}

// sleepCtx ждёт d, но прерывается по отмене контекста.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
