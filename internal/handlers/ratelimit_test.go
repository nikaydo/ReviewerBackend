package handlers

import (
	"testing"
	"time"
)

func TestLimiterAllowsUpToLimit(t *testing.T) {
	l := newLimiter(3, 60000) // Окно достаточно длинное, чтобы счётчик не сбрасывался.

	for i := 1; i <= 3; i++ {
		if ok, _ := l.allow("1.2.3.4"); !ok {
			t.Fatalf("запрос %d из 3 отклонён", i)
		}
	}

	ok, retryAfter := l.allow("1.2.3.4")
	if ok {
		t.Fatal("четвёртый запрос должен быть отклонён")
	}
	if retryAfter <= 0 {
		t.Errorf("retryAfter = %v, ожидалось положительное значение", retryAfter)
	}
}

func TestLimiterIsolatesSources(t *testing.T) {
	l := newLimiter(1, 60000)

	if ok, _ := l.allow("1.1.1.1"); !ok {
		t.Fatal("первый запрос от 1.1.1.1 отклонён")
	}
	// Исчерпание лимита одного адреса не должно влиять на другие.
	if ok, _ := l.allow("2.2.2.2"); !ok {
		t.Fatal("первый запрос от 2.2.2.2 отклонён")
	}
	if ok, _ := l.allow("1.1.1.1"); ok {
		t.Fatal("второй запрос от 1.1.1.1 должен быть отклонён")
	}
}

func TestLimiterWindowResets(t *testing.T) {
	l := newLimiter(1, 1) // Окно в 1 наносекунду.

	if ok, _ := l.allow("1.2.3.4"); !ok {
		t.Fatal("первый запрос отклонён")
	}
	// Окно успело истечь, поэтому счётчик должен сброситься.
	if ok, _ := l.allow("1.2.3.4"); !ok {
		t.Fatal("после истечения окна запрос должен пройти")
	}
}

func TestLimiterReset(t *testing.T) {
	l := newLimiter(2, 60000)

	l.allow("1.2.3.4")
	l.allow("1.2.3.4")
	if ok, _ := l.allow("1.2.3.4"); ok {
		t.Fatal("третий запрос должен быть отклонён")
	}

	l.reset("1.2.3.4")
	if ok, _ := l.allow("1.2.3.4"); !ok {
		t.Fatal("после reset запрос должен пройти")
	}
}

func TestLimiterBoundsMemory(t *testing.T) {
	l := newLimiter(1, 1)

	// Много разных источников: карта не должна расти бесконечно.
	for i := 0; i < maxLimiterEntries*2; i++ {
		l.allow(randomKey(i))
	}

	l.mu.Lock()
	size := len(l.entries)
	l.mu.Unlock()

	if size > maxLimiterEntries {
		t.Errorf("размер карты %d, ожидалось не больше %d", size, maxLimiterEntries)
	}
}

// randomKey формирует уникальный ключ источника для теста памяти.
func randomKey(i int) string {
	const digits = "0123456789"
	buf := make([]byte, 12)
	for pos := range buf {
		buf[pos] = digits[(i+pos)%10]
	}
	return string(buf)
}

func TestFormatSeconds(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"нулевой интервал округляется до секунды", 0, "1"},
		{"отрицательный интервал округляется до секунды", -5 * time.Second, "1"},
		{"дробная секунда округляется вниз", 1500 * time.Millisecond, "1"},
		{"целые секунды", 9 * time.Second, "9"},
		{"минуты", 90 * time.Second, "90"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSeconds(tt.d); got != tt.want {
				t.Errorf("formatSeconds(%v) = %q, ожидалось %q", tt.d, got, tt.want)
			}
		})
	}
}
