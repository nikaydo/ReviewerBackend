// Package queue организует асинхронную генерацию отзывов.
//
// Рабочие построены на каналах, а не на опросе списка по таймеру. В исходной
// версии очередь была обычным срезом в памяти, который воркер просматривал
// каждые 500 мс, просыпался даже при пустой очереди и тратил процессор;
// удаление задания искало его по идентификатору пользователя, а не запроса,
// из-за чего один завершившийся запрос удалял все ожидающие запросы того же
// пользователя.
//
// Здесь каждый запрос — отдельная задача с собственным идентификатором,
// воркеры берут задания из канала, а завершение задачи сигнализируется
// отдельным каналом. Снимок состояния для интерфейса берётся под блокировкой.
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/nikaydo/reviewer/internal/ai"
	"github.com/nikaydo/reviewer/internal/database"
)

// Тип задания.
const (
	// TargetReview — сгенерировать отзыв.
	TargetReview = "review"
	// TargetTitle — сгенерировать заголовок к отзыву.
	TargetTitle = "title"
)

// ErrQueueClosed возвращается при попытке добавить задание в закрытую очередь.
var ErrQueueClosed = errors.New("очередь закрыта")

// Job — задание на генерацию.
type Job struct {
	// ID идентифицирует задание: по нему воркер убирает его из очереди,
	// а интерфейс — опрашивает состояние.
	ID uuid.UUID
	// UserID — владелец задания.
	UserID string
	// ReviewID — отзыв, к которому относится задание. Пуст для нового отзыва.
	ReviewID string
	// Target — что именно нужно сгенерировать.
	Target string
	// Model — идентификатор модели в формате OpenRouter.
	Model string
	// Request — текст запроса пользователя.
	Request string
	// SystemPrompt — собранный системный промт.
	SystemPrompt string
	// AssistantHistory — предыдущий ответ модели для продолжения диалога.
	AssistantHistory string
	// ReasoningEffort — глубина размышления для моделей с reasoning.
	ReasoningEffort string
	// UseMemory — нужно ли обновить память пользователя по итогу генерации.
	UseMemory bool

	createdAt time.Time
}

// Position — положение задания в очереди для интерфейса.
type Position struct {
	// Position — номер задания среди ожидающих, начиная с 1. Ноль означает,
	// что задание не найдено: оно уже выполнено, отменено или ещё не принято.
	Position int `json:"Position"`
	// Total — сколько заданий ожидает.
	Total int `json:"Total"`
}

// Generator выполняет задание. Интерфейс позволяет подменить реальный
// вызов OpenRouter в тестах.
type Generator interface {
	Generate(ctx context.Context, req ai.Request) (ai.Answer, error)
}

// Config — параметры очереди.
type Config struct {
	// Workers — сколько генераций выполняется одновременно.
	Workers int
	// BetweenRequests — пауза между отправкой запросов к API внутри одного
	// воркера: ограничивает частоту обращений к провайдеру.
	BetweenRequests time.Duration
	// Retention — сколько хранить завершённые задания, чтобы интерфейс мог
	// узнать результат.
	Retention time.Duration
}

// Queue — очередь генерации.
type Queue struct {
	jobs chan Job
	// stop останавливает воркеров при Close. Отдельный канал нужен,
	// потому что отмена контекста задаётся вызывающей стороной, а Close
	// обязан завершаться и без неё.
	stop    chan struct{}
	done    chan struct{}
	cfg     Config
	gen     Generator
	store   *database.Store
	prompts PromptProvider
	log     *slog.Logger

	// pending хранит ожидающие задания. Нужен для ответа на запрос
	// о положении в очереди.
	mu      sync.RWMutex
	pending map[uuid.UUID]Job

	// finished хранит недавно завершённые задания с итоговой ошибкой.
	finishedMu sync.RWMutex
	finished   map[uuid.UUID]error

	closed atomic.Bool
	wg     sync.WaitGroup
}

// PromptProvider отдаёт промт для формирования памяти.
type PromptProvider interface {
	MemorizationPrompt(ctx context.Context, userID string) (string, error)
}

// New создаёт очередь.
func New(cfg Config, gen Generator, store *database.Store, prompts PromptProvider, log *slog.Logger) *Queue {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 10 * time.Minute
	}
	if log == nil {
		log = slog.Default()
	}
	return &Queue{
		// Буфер небольшой: очередь ограничена параллелизмом воркеров,
		// лишние запросы должны получать отказ, а не копиться без предела.
		jobs:     make(chan Job, cfg.Workers*4),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		cfg:      cfg,
		gen:      gen,
		store:    store,
		prompts:  prompts,
		log:      log,
		pending:  make(map[uuid.UUID]Job),
		finished: make(map[uuid.UUID]error),
	}
}

// SetPromptProvider задаёт источник промта для памяти.
//
// Задаётся отдельно от New, потому что источник живёт в слое обработчиков,
// который сам зависит от очереди: так снимается циклическая зависимость
// между пакетами.
func (q *Queue) SetPromptProvider(p PromptProvider) {
	q.prompts = p
}

// Start запускает воркеров и очистку завершённых заданий.
func (q *Queue) Start(ctx context.Context) {
	for i := 0; i < q.cfg.Workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx, i)
	}
	q.wg.Add(1)
	go q.janitor(ctx)
}

// Submit принимает задание в обработку.
//
// Блокировка send с таймаутом не даёт клиенту висеть вечно, если очередь
// переполнена и воркеры не успевают.
func (q *Queue) Submit(ctx context.Context, job Job) (uuid.UUID, error) {
	// Проверяем отмену до всех остальных условий. Иначе select с готовыми
	// каналами jobs и ctx.Done() выбирал бы между ними случайно, и
	// отменённый запрос изредка принимался бы в очередь.
	if err := ctx.Err(); err != nil {
		return uuid.Nil, err
	}
	if q.closed.Load() {
		return uuid.Nil, ErrQueueClosed
	}
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	job.createdAt = time.Now()

	q.mu.Lock()
	q.pending[job.ID] = job
	q.mu.Unlock()

	select {
	case q.jobs <- job:
		return job.ID, nil
	case <-ctx.Done():
		q.removePending(job.ID)
		return uuid.Nil, ctx.Err()
	case <-q.done:
		q.removePending(job.ID)
		return uuid.Nil, ErrQueueClosed
	}
}

// worker обрабатывает задания из канала.
func (q *Queue) worker(ctx context.Context, index int) {
	defer q.wg.Done()
	log := q.log.With("worker", index)

	for {
		select {
		case <-ctx.Done():
			log.Info("воркер остановлен")
			return
		case <-q.stop:
			log.Info("воркер остановлен")
			return
		case job, ok := <-q.jobs:
			if !ok {
				return
			}
			q.process(ctx, log, job)

			// Пауза между запросами одного воркера: так N воркеров не
			// устраивают залп из N одновременных обращений к API.
			if q.cfg.BetweenRequests > 0 {
				select {
				case <-time.After(q.cfg.BetweenRequests):
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// process выполняет одно задание.
//
// Паника внутри задания перехватывается. Без этого единственный сбойный
// запрос (например, из-за неожиданных данных от провайдера) завершал бы весь
// процесс: middleware.Recoverer спасает только HTTP-обработчики, а воркеры
// очереди работают отдельными горутинами.
func (q *Queue) process(ctx context.Context, log *slog.Logger, job Job) {
	defer q.removePending(job.ID)
	defer func() {
		if r := recover(); r != nil {
			// Ошибка времени выполнения записывается вместе с местом,
			// где она возникла: без стека причина обычно неочевидна.
			log.Error("паника при обработке задания",
				"job", job.ID,
				"user", job.UserID,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			q.markFinished(job.ID, fmt.Errorf("внутренняя ошибка обработки задания"))
			q.clearInProgress(ctx, job)
		}
	}()

	started := time.Now()
	answer, err := q.gen.Generate(ctx, ai.Request{
		Model:            job.Model,
		UserInput:        job.Request,
		System:           job.SystemPrompt,
		AssistantHistory: job.AssistantHistory,
		ReasoningEffort:  job.ReasoningEffort,
	})
	if err != nil {
		log.Error("генерация не удалась",
			"job", job.ID, "user", job.UserID, "target", job.Target, "err", err)
		q.markFinished(job.ID, err)
		q.clearInProgress(ctx, job)
		return
	}

	if err := q.persist(ctx, job, answer); err != nil {
		log.Error("не удалось сохранить результат",
			"job", job.ID, "user", job.UserID, "err", err)
		q.markFinished(job.ID, err)
		q.clearInProgress(ctx, job)
		return
	}

	if job.UseMemory {
		q.updateMemory(ctx, log, job, answer.Text)
	}

	q.markFinished(job.ID, nil)
	q.clearInProgress(ctx, job)

	log.Info("задание выполнено",
		"job", job.ID,
		"user", job.UserID,
		"target", job.Target,
		"model", answer.Model,
		"duration", time.Since(started).Round(time.Millisecond),
		"prompt_tokens", answer.PromptTokens,
		"completion_tokens", answer.CompletionTokens,
	)
}

// persist сохраняет результат генерации.
func (q *Queue) persist(ctx context.Context, job Job, answer ai.Answer) error {
	if q.store == nil {
		return errors.New("хранилище не настроено")
	}
	switch job.Target {
	case TargetTitle:
		return q.store.UpsertReviewTitle(ctx, job.UserID, job.ReviewID, answer.Text, job.Request)
	case TargetReview:
		_, err := q.store.CreateReview(ctx, database.Review{
			UserID:    job.UserID,
			Request:   job.Request,
			Answer:    answer.Text,
			Reasoning: answer.Reasoning,
			Model:     answer.Model,
		})
		return err
	default:
		return fmt.Errorf("неизвестный тип задания %q", job.Target)
	}
}

// updateMemory дополняет память пользователя по итогам генерации.
func (q *Queue) updateMemory(ctx context.Context, log *slog.Logger, job Job, reviewText string) {
	if q.prompts == nil || q.store == nil {
		return
	}

	prompt, err := q.prompts.MemorizationPrompt(ctx, job.UserID)
	if err != nil {
		log.Error("не удалось получить промт для памяти", "job", job.ID, "err", err)
		return
	}

	current, err := q.store.RecallMemory(ctx, job.UserID)
	if err != nil {
		log.Error("не удалось прочитать память", "job", job.ID, "err", err)
		return
	}

	answer, err := q.gen.Generate(ctx, ai.Request{
		Model:     job.Model,
		UserInput: reviewText,
		System:    MemorizationPrompt(prompt, current),
	})
	if err != nil {
		log.Error("не удалось обновить память", "job", job.ID, "err", err)
		return
	}

	if err := q.store.RememberMemory(ctx, job.UserID, answer.Text); err != nil {
		log.Error("не удалось сохранить память", "job", job.ID, "err", err)
	}
}

// clearInProgress сбрасывает признак незавершённой генерации.
func (q *Queue) clearInProgress(ctx context.Context, job Job) {
	if q.store == nil {
		return
	}
	if err := q.store.SetInProgress(ctx, job.UserID, ""); err != nil && !errors.Is(err, database.ErrNotFound) {
		q.log.Error("не удалось сбросить состояние запроса", "job", job.ID, "err", err)
	}
}

// Position возвращает положение задания в очереди.
func (q *Queue) Position(id uuid.UUID) Position {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if _, ok := q.pending[id]; !ok {
		return Position{}
	}

	// Порядок в очереди задаётся временем постановки в неё: map не упорядочен,
	// поэтому задания приходится отсортировать перед нумерацией.
	ordered := make([]Job, 0, len(q.pending))
	for _, job := range q.pending {
		ordered = append(ordered, job)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].createdAt.Before(ordered[j].createdAt)
	})

	for i, job := range ordered {
		if job.ID == id {
			return Position{Position: i + 1, Total: len(ordered)}
		}
	}
	return Position{}
}

// ErrFor возвращает ошибку завершения задания, если оно уже завершилось
// с ошибкой. Для успешных и ещё не завершённых заданий возвращается nil.
func (q *Queue) ErrFor(id uuid.UUID) error {
	q.finishedMu.RLock()
	defer q.finishedMu.RUnlock()
	return q.finished[id]
}

// Waiting возвращает число ожидающих заданий.
func (q *Queue) Waiting() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.pending)
}

// removePending убирает задание из списка ожидающих.
func (q *Queue) removePending(id uuid.UUID) {
	q.mu.Lock()
	delete(q.pending, id)
	q.mu.Unlock()
}

// markFinished запоминает итог задания.
func (q *Queue) markFinished(id uuid.UUID, err error) {
	q.finishedMu.Lock()
	q.finished[id] = err
	q.finishedMu.Unlock()
}

// janitor удаляет завершённые задания, чтобы карта не росла бесконечно.
func (q *Queue) janitor(ctx context.Context) {
	defer q.wg.Done()

	interval := q.cfg.Retention / 2
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.stop:
			return
		case <-ticker.C:
			q.finishedMu.Lock()
			q.finished = make(map[uuid.UUID]error)
			q.finishedMu.Unlock()
		}
	}
}

// Close останавливает очередь и ждёт завершения воркеров.
//
// Вызывать нужно ровно один раз; повторный вызов безопасен и ничего не
// делает.
func (q *Queue) Close() {
	if q.closed.Swap(true) {
		return
	}
	close(q.stop)
	q.wg.Wait()
}

// memorySlot — маркер, вместо которого в промт подставляется текущая память.
const memorySlot = "<memory>"

// MemorizationPrompt возвращает шаблон промта с уже подставленной памятью.
func MemorizationPrompt(template, current string) string {
	if current == "" {
		return template
	}
	if strings.Contains(template, memorySlot) {
		return strings.Replace(template, memorySlot, current, 1)
	}
	// Маркера в шаблоне нет: добавляем память в конец, чтобы она точно
	// попала в запрос.
	return template + "\n\nТекущая память о пользователе: " + current
}
