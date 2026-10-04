package jwt

import (
	"errors"
	"testing"
	"time"
)

const (
	testSecret = "secret-for-tests-0123456789-0123456789"
	testIssuer = "test-issuer"
)

func newTestManager() *Manager {
	m := NewManager(testSecret, testIssuer, time.Hour)
	return m
}

func TestNewAccessTokenAndParse(t *testing.T) {
	m := newTestManager()

	token, expiresAt, err := m.NewAccessToken("user-123", "nikaydo", RoleViewer)
	if err != nil {
		t.Fatalf("NewAccessToken вернул ошибку: %v", err)
	}
	if token == "" {
		t.Fatal("токен пуст")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("срок истечения %v уже прошёл", expiresAt)
	}

	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse вернул ошибку: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, ожидалось user-123", claims.Subject)
	}
	if claims.Login != "nikaydo" {
		t.Errorf("Login = %q, ожидалось nikaydo", claims.Login)
	}
	if claims.Role != RoleViewer {
		t.Errorf("Role = %q, ожидалось %s", claims.Role, RoleViewer)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	token, _, err := newTestManager().NewAccessToken("user-123", "nikaydo", RoleViewer)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	other := NewManager("other-secret-0123456789-0123456789", testIssuer, time.Hour)
	if _, err := other.Parse(token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Parse вернул %v, ожидалась ErrTokenInvalid", err)
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	token, _, err := newTestManager().NewAccessToken("user-123", "nikaydo", RoleViewer)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	other := NewManager(testSecret, "someone-else", time.Hour)
	if _, err := other.Parse(token); err == nil {
		t.Fatal("токен с чужим issuer не должен приниматься")
	}
}

func TestParseDetectsExpired(t *testing.T) {
	m := newTestManager()

	token, _, err := m.NewAccessToken("user-123", "nikaydo", RoleViewer)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	// Сдвигаем время вперёд: токен, выпущенный час назад, истёк.
	m.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	if _, err := m.Parse(token); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("Parse вернул %v, ожидалась ErrTokenExpired", err)
	}
}

func TestParseRejectsUnsignedToken(t *testing.T) {
	// Токен с alg=none — классическая атака на подмену алгоритма.
	// Подпись HMAC не проверяется, если алгоритм не задан.
	const noneToken = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJhdHRhY2tlciIsImV4cCI6OTk5OTk5OTk5OX0."

	if _, err := newTestManager().Parse(noneToken); err == nil {
		t.Fatal("токен с alg=none не должен приниматься")
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"пустая строка", ""},
		{"не JWT", "просто-текст"},
		{"две части", "aaa.bbb"},
		{"мусор в подписи", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ4In0.мусор"},
	}

	m := newTestManager()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := m.Parse(tt.token); err == nil {
				t.Fatal("ожидалась ошибка разбора")
			}
		})
	}
}

func TestParseRejectsMissingSubject(t *testing.T) {
	m := newTestManager()

	// Выпускаем токен без subject: подпись верна, но идентификатора нет.
	token, _, err := m.NewAccessToken("", "nikaydo", RoleViewer)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	if _, err := m.Parse(token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Parse вернул %v, ожидалась ErrTokenInvalid", err)
	}
}

func TestTokensAreUnique(t *testing.T) {
	m := newTestManager()

	seen := make(map[string]struct{}, 50)
	for i := 0; i < 50; i++ {
		token, _, err := m.NewAccessToken("user-123", "nikaydo", RoleViewer)
		if err != nil {
			t.Fatalf("NewAccessToken: %v", err)
		}
		if _, dup := seen[token]; dup {
			t.Fatal("токен повторился")
		}
		seen[token] = struct{}{}
	}
}
