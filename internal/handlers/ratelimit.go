package handlers

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// limiter — скользящее окно, ограничивающее число запросов от одного
// источника.
//
// Применяется к эндпоинтам регистрации и входа: без него можно было бы
// перебирать пароли или плодить аккаунты.
// Основан на обычном счётчике с окном, а не на внешнем хранилище: держать
// состояние в памяти достаточно, потому что при перезапуске процесса
// сбрасывать счётчики безопасно.
type limiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	entries map[string]*window
	lastGC  time.Time
}

// window — счётчик запросов для одного источника.
type window struct {
	count int
	// started — начало текущего окна.
	started time.Time
}

// newLimiter создаёт ограничитель.
func newLimiter(limit int, w time.Duration) *limiter {
	if limit < 1 {
		limit = 1
	}
	if w <= 0 {
		w = time.Minute
	}
	return &limiter{
		limit:  limit,
		window: w,
		// Ограничение размера карты: иначе перебором IP-адресов можно
		// было бы раздуть память процесса.
		entries: make(map[string]*window),
		lastGC:  time.Now(),
	}
}

// maxLimiterEntries ограничивает число отслеживаемых источников.
const maxLimiterEntries = 10000

// allow сообщает, можно ли выполнить запрос от указанного источника.
func (l *limiter) allow(key string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.gcLocked(now)

	w, ok := l.entries[key]
	if !ok {
		l.entries[key] = &window{count: 1, started: now}
		return true, 0
	}

	if now.Sub(w.started) >= l.window {
		w.count = 1
		w.started = now
		return true, 0
	}

	if w.count >= l.limit {
		retryAfter := l.window - now.Sub(w.started)
		return false, retryAfter
	}

	w.count++
	return true, 0
}

// reset сбрасывает счётчик источника.
//
// Вызывается после успешного входа, чтобы легитимный пользователь не
// исчерпал лимит на самом себе.
func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[key] = &window{count: 0, started: time.Now()}
}

// gcLocked удаляет истёкшие окна. Вызывается под блокировкой.
func (l *limiter) gcLocked(now time.Time) {
	if now.Sub(l.lastGC) < l.window {
		return
	}
	l.lastGC = now
	for k, w := range l.entries {
		if now.Sub(w.started) >= l.window {
			delete(l.entries, k)
		}
	}
	// Если всё равно много записей, сбрасываем карту целиком: точность
	// счётчиков не настолько важна, сколько важно не терять память.
	if len(l.entries) > maxLimiterEntries {
		l.entries = make(map[string]*window)
	}
}

// LimitAuth ограничивает частоту запросов к эндпоинтам авторизации.
func (h *Handlers) LimitAuth(next http.Handler) http.Handler {
	if h.authLimiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		key := clientKey(r)
		if ok, retryAfter := h.authLimiter.allow(key); !ok {
			w.Header().Set("Retry-After", formatSeconds(retryAfter))
			fail(ctx, w, h.Log, http.StatusTooManyRequests, "rate_limited",
				"слишком много попыток, попробуйте позже")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientKey определяет источник запроса.
//
// Доверяем заголовку X-Real-IP, который выставляет middleware.RealIP:
// за обратным прокси значение RemoteAddr — это адрес прокси, и без заголовка
// все запросы попали бы в один счётчик.
func clientKey(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// formatSeconds округляет интервал до целых секунд с положительным минимумом.
func formatSeconds(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}
