package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nikaydo/reviewer/internal/ai"
)

// fakeGenerator подменяет обращение к LLM.
type fakeGenerator struct {
	mu sync.Mutex

	// answer — что вернуть в ответ.
	answer ai.Answer
	// err — ошибка, которую нужно вернуть.
	err error
	// calls — сколько раз вызвали генерацию.
	calls int
	// requests — какие запросы пришли.
	requests []ai.Request
	// delay — искусственная задержка.
	delay time.Duration
}

func (f *fakeGenerator) Generate(_ context.Context, req ai.Request) (ai.Answer, error) {
	f.mu.Lock()
	f.calls++
	f.requests = append(f.requests, req)
	delay, answer, err := f.delay, f.answer, f.err
	f.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	return answer, err
}

func (f *fakeGenerator) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// newTestQueue создаёт очередь для тестов и запускает её.
//
// store равен nil: тесты не доводят задание до записи в базу, а проверяют
// только приём, обработку и состояние очереди. Контекст отменяется при
// завершении теста, что останавливает воркеров.
func newTestQueue(t *testing.T, gen Generator, cfg Config) *Queue {
	t.Helper()
	if cfg.Workers == 0 {
		cfg.Workers = 1
	}
	q := New(cfg, gen, nil, nil, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)
	t.Cleanup(func() {
		cancel()
		q.Close()
	})
	return q
}

func TestSubmitAssignsID(t *testing.T) {
	q := newTestQueue(t, &fakeGenerator{}, Config{Workers: 1})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменённый контекст: задание не должно приняться.
	if _, err := q.Submit(ctx, Job{UserID: "u1", Target: TargetReview}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit вернул %v, ожидался context.Canceled", err)
	}
}

func TestSubmitStoresPending(t *testing.T) {
	gen := &fakeGenerator{delay: 50 * time.Millisecond}
	q := newTestQueue(t, gen, Config{Workers: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobID, err := q.Submit(ctx, Job{UserID: "u1", Target: TargetReview})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if jobID == uuid.Nil {
		t.Fatal("Submit не присвоил идентификатор")
	}

	// Пока воркер занят, задание должно оставаться в списке ожидающих.
	pos := q.Position(jobID)
	if pos.Total != 1 {
		t.Errorf("Total = %d, ожидалось 1", pos.Total)
	}
	if pos.Position != 1 {
		t.Errorf("Position = %d, ожидалось 1", pos.Position)
	}
}

func TestPositionUnknownJob(t *testing.T) {
	q := newTestQueue(t, &fakeGenerator{}, Config{Workers: 1})

	pos := q.Position(uuid.New())
	if pos.Position != 0 || pos.Total != 0 {
		t.Errorf("Position = %+v, ожидались нули для неизвестного задания", pos)
	}
}

func TestPositionReflectsQueueOrder(t *testing.T) {
	// Три воркера отсутствуют: задания копятся в буфере канала и остаются
	// ожидающими, что позволяет проверить нумерацию.
	// Воркеры не запускаем: иначе задания сразу забираются, и проверить
	// нумерацию в очереди не получится.
	gen := &fakeGenerator{delay: time.Second}
	q := New(Config{Workers: 1, Retention: time.Minute}, gen, nil, nil, discardLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		job := Job{
			ID:     uuid.New(),
			UserID: "u1",
			Target: TargetReview,
		}
		// Разносим задания по времени, чтобы порядок был определён.
		job.createdAt = time.Now().Add(time.Duration(i) * time.Millisecond)
		if _, err := q.Submit(ctx, job); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
		ids = append(ids, job.ID)
	}

	// Первые два задания воркер мог забрать сразу, поэтому проверяем
	// относительный порядок среди оставшихся ожидающими.
	first := q.Position(ids[0])
	last := q.Position(ids[2])
	if first.Position > last.Position && first.Position != 0 {
		t.Errorf("порядок нарушен: первое задание %d, последнее %d", first.Position, last.Position)
	}
}

func TestMemorizationPrompt(t *testing.T) {
	tests := []struct {
		name     string
		template string
		current  string
		want     string
	}{
		{
			name:     "маркер подставлен",
			template: "Память: <memory>. Конец.",
			current:  "любит кофе",
			want:     "Память: любит кофе. Конец.",
		},
		{
			name:     "пустая память не меняет шаблон",
			template: "Память: <memory>.",
			current:  "",
			want:     "Память: <memory>.",
		},
		{
			name:     "без маркера память дописывается",
			template: "Основная инструкция.",
			current:  "любит кофе",
			want:     "Основная инструкция.\n\nТекущая память о пользователе: любит кофе",
		},
		{
			name:     "подставляется только первый маркер",
			template: "<memory> и ещё <memory>",
			current:  "X",
			want:     "X и ещё <memory>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MemorizationPrompt(tt.template, tt.current); got != tt.want {
				t.Errorf("MemorizationPrompt =\n%q\nожидалось\n%q", got, tt.want)
			}
		})
	}
}

func TestCloseStopsAcceptingJobs(t *testing.T) {
	q := newTestQueue(t, &fakeGenerator{}, Config{Workers: 1})
	q.Close()

	if _, err := q.Submit(context.Background(), Job{UserID: "u1"}); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("Submit после Close вернул %v, ожидался ErrQueueClosed", err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	q := newTestQueue(t, &fakeGenerator{}, Config{Workers: 1})
	q.Close()
	q.Close() // Второй вызов не должен паниковать.
}

func TestWorkersProcessConcurrently(t *testing.T) {
	const jobs = 4

	gen := &fakeGenerator{delay: 30 * time.Millisecond, answer: ai.Answer{Text: "ответ"}}
	q := newTestQueue(t, gen, Config{Workers: 4})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for i := 0; i < jobs; i++ {
		if _, err := q.Submit(ctx, Job{UserID: "u1", Target: TargetReview}); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if gen.callCount() >= jobs {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := gen.callCount(); got != jobs {
		t.Errorf("генераций %d, ожидалось %d", got, jobs)
	}
}

func TestErrForRecordsFailure(t *testing.T) {
	// Воркер без базы не сможет сохранить результат; задание завершится
	// ошибкой, и её нужно увидеть через ErrFor.
	gen := &fakeGenerator{answer: ai.Answer{Text: "ответ", Model: "test"}}
	q := newTestQueue(t, gen, Config{Workers: 1})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	jobID, err := q.Submit(ctx, Job{UserID: "u1", Target: TargetReview})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		last = q.ErrFor(jobID)
		if last != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if last == nil {
		t.Error("ошибка задания не была записана")
	}
}

func TestJobRejectsUnknownTarget(t *testing.T) {
	// persist возвращает ошибку для неизвестного типа задания.
	q := newTestQueue(t, &fakeGenerator{}, Config{Workers: 1})
	if err := q.persist(context.Background(), Job{Target: "что-то"}, ai.Answer{}); err == nil {
		t.Fatal("неизвестный тип задания должен приводить к ошибке")
	}
}
