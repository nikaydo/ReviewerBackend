package ai

import (
	"fmt"
	"strings"
)

// Model описывает модель, доступную через OpenRouter.
type Model struct {
	// ID — идентификатор модели в формате OpenRouter ("provider/model").
	ID string
	// Title — название для интерфейса.
	Title string
	// Reasoning помечает модели с режимом размышления: их ответ приходит
	// в отдельном поле reasoning, а не в общем тексте.
	Reasoning bool
	// Recommended выделяет модель в списке как основную.
	Recommended bool
}

// Models — разрешённые модели.
//
// Список закрытый намеренно. Раньше идентификатор модели приходил из формы
// и уходил в API как есть, поэтому любой пользователь мог указать
// сколь угодно дорогую модель и потратить чужие деньги. Теперь запрос
// отклоняется, если модели нет в этом списке.
var Models = []Model{
	{
		ID:          "anthropic/claude-sonnet-4.6",
		Title:       "Claude Sonnet 4.6",
		Reasoning:   true,
		Recommended: true,
	},
	{
		ID:          "google/gemini-2.5-flash",
		Title:       "Gemini 2.5 Flash",
		Reasoning:   true,
		Recommended: true,
	},
	{
		ID:          "mistralai/mistral-large-2512",
		Title:       "Mistral Large 3",
		Recommended: true,
	},
	{
		ID:          "deepseek/deepseek-chat-v3.1",
		Title:       "DeepSeek V3.1",
		Reasoning:   true,
		Recommended: true,
	},
	{
		ID:        "openai/gpt-4o-mini",
		Title:     "GPT-4o mini",
		Reasoning: false,
	},
}

// DefaultModel используется, если модель не указана.
const DefaultModel = "anthropic/claude-sonnet-4.6"

// ErrUnknownModel возвращается, если запрошенной модели нет в списке.
type ErrUnknownModel struct {
	ID string
}

func (e *ErrUnknownModel) Error() string {
	return fmt.Sprintf("модель %q недоступна", e.ID)
}

// ResolveModel возвращает описание разрешённой модели.
//
// Пустое имя трактуется как выбор модели по умолчанию: так клиенты,
// не отправляющие поле model, продолжают работать.
func ResolveModel(id string) (Model, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = DefaultModel
	}
	for _, m := range Models {
		if m.ID == id {
			return m, nil
		}
	}
	return Model{}, &ErrUnknownModel{ID: id}
}

// IsAllowed сообщает, входит ли модель в разрешённый список.
func IsAllowed(id string) bool {
	_, err := ResolveModel(id)
	return err == nil
}
