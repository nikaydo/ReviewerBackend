package auth

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestMain снижает стоимость bcrypt на время тестов.
//
// При стоимости 12 каждый хеш занимает около 250 мс, и набор тестов
// растягивается на десятки секунд — особенно под -race. Проверяемая логика
// от стоимости не зависит, поэтому в тестах достаточно минимальной.
func TestMain(m *testing.M) {
	Cost = bcrypt.MinCost
	os.Exit(m.Run())
}

func TestHashPasswordProducesUsableHash(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("HashPassword вернул ошибку: %v", err)
	}
	if hash == "correct-horse-battery" {
		t.Fatal("пароль сохранён в открытом виде")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("ожидался bcrypt-хеш, получено %q", hash)
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	const password = "correct-horse-battery"

	first, err := HashPassword(password)
	if err != nil {
		t.Fatalf("первый хеш: %v", err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatalf("второй хеш: %v", err)
	}

	// Соль в bcrypt встроена, поэтому хеши одного пароля различаются.
	// Одинаковые хеши означали бы, что соль не используется.
	if first == second {
		t.Fatal("два хеша одного пароля совпали: соль не работает")
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	tests := []struct {
		name       string
		stored     string
		password   string
		wantOK     bool
		wantLegacy bool
	}{
		{
			name:     "верный пароль",
			stored:   hash,
			password: "correct-horse-battery",
			wantOK:   true,
		},
		{
			name:     "неверный пароль",
			stored:   hash,
			password: "wrong-password",
			wantOK:   false,
		},
		{
			name:     "пустой пароль",
			stored:   hash,
			password: "",
			wantOK:   false,
		},
		{
			name:     "пустой хеш",
			stored:   "",
			password: "correct-horse-battery",
			wantOK:   false,
		},
		{
			name:       "пароль из старого формата",
			stored:     "plaintext-password",
			password:   "plaintext-password",
			wantOK:     true,
			wantLegacy: true,
		},
		{
			// Признак legacy описывает формат хранения, а не результат
			// проверки: строка в базе открытым текстом в любом случае
			// требует перевода на хеш. Вызывающий код смотрит на флаг
			// только при успешной проверке.
			name:       "неверный пароль при старом формате",
			stored:     "plaintext-password",
			password:   "other-password",
			wantOK:     false,
			wantLegacy: true,
		},
		{
			// Значение выглядит как bcrypt по длине, но не является хешем.
			// Сравнивать его как открытый текст нельзя: так злоумышленник,
			// записавший в это поле произвольную строку, получил бы
			// возможность войти, указав её же в качестве пароля.
			name:       "похожий на bcrypt, но испорченный",
			stored:     "$2a$12$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			password:   "anything",
			wantOK:     false,
			wantLegacy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, legacy := VerifyPassword(tt.stored, tt.password)
			if ok != tt.wantOK {
				t.Errorf("VerifyPassword = %v, ожидалось %v", ok, tt.wantOK)
			}
			if legacy != tt.wantLegacy {
				t.Errorf("legacy = %v, ожидалось %v", legacy, tt.wantLegacy)
			}
		})
	}
}

func TestNewRefreshTokenShape(t *testing.T) {
	token, hash, family, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken вернул ошибку: %v", err)
	}

	if !strings.HasPrefix(token, family+".") {
		t.Fatalf("токен %q должен начинаться с идентификатора семейства %q", token, family)
	}
	if len(family) < 20 {
		t.Errorf("идентификатор семейства слишком короткий: %q", family)
	}

	// Секретная часть должна быть достаточно длинной.
	secret := strings.TrimPrefix(token, family+".")
	if len(secret) < 40 {
		t.Errorf("секретная часть слишком короткая: %d символов", len(secret))
	}

	if hash == token {
		t.Fatal("хеш совпадает с самим токеном")
	}
	if HashRefreshToken(token) != hash {
		t.Fatal("хеш не воспроизводится")
	}
}

func TestNewRefreshTokenIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		token, _, family, err := NewRefreshToken()
		if err != nil {
			t.Fatalf("NewRefreshToken: %v", err)
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("токен повторился на итерации %d", i)
		}
		// Семейства тоже должны различаться: иначе ротация склеила бы
		// разные сессии в одну.
		if _, dup := seen[family]; dup {
			t.Fatalf("идентификатор семейства повторился на итерации %d", i)
		}
		seen[token] = struct{}{}
		seen[family] = struct{}{}
	}
}

func TestMatchesRefreshToken(t *testing.T) {
	token, hash, _, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}

	if !MatchesRefreshToken(hash, token) {
		t.Error("верный токен должен совпадать со своим хешем")
	}
	if MatchesRefreshToken(hash, token+"x") {
		t.Error("изменённый токен не должен совпадать")
	}
	if MatchesRefreshToken("", token) {
		t.Error("пустой хеш не должен совпадать ни с чем")
	}
	if MatchesRefreshToken(hash, "") {
		t.Error("пустой токен не должен совпадать")
	}
}

func TestTokenFamily(t *testing.T) {
	token, _, family, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	if got := TokenFamily(token); got != family {
		t.Errorf("TokenFamily = %q, ожидалось %q", got, family)
	}
	if got := TokenFamily("нет-точки"); got != "" {
		t.Errorf("TokenFamily без разделителя = %q, ожидалась пустая строка", got)
	}
	if got := TokenFamily(""); got != "" {
		t.Errorf("TokenFamily от пустой строки = %q", got)
	}
}

func TestDummyVerifyDoesNotPanic(t *testing.T) {
	// Функция ничего не возвращает; проверяем лишь, что она безопасна
	// и не падает на пустом вводе.
	DummyVerify("")
	DummyVerify("anything")
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("HashPassword должен отклонять пустой пароль")
	}
}
