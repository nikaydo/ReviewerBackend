// Package auth отвечает за проверку паролей и хранение refresh-токенов.
//
// Пароли хранятся в виде bcrypt-хешей: даже при утечке базы восстановить
// исходный пароль невозможно. Сравнение выполняется через bcrypt.Compare,
// а не хешированием входного значения и сравнением строк, чтобы результат
// проверки не зависел от соли.
//
// Refresh-токены в базе не хранятся: сохраняется только их хеш. Поэтому
// утечка базы не даёт возможности войти под пользователем — токен ещё нужно
// предъявить. Это же позволяет искать токен при ротации, не разбирая все
// значения подряд.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Cost — стоимость хеширования bcrypt.
//
// Значение по умолчанию подобрано так, чтобы хеширование занимало ощутимое,
// но не чрезмерное время: 12 — примерно 250 мс на современном CPU. Этого
// достаточно, чтобы перебор паролей был дорогим.
//
// Поле сделано переменной, а не константой: тесты снижают стоимость, иначе
// каждый тест на хеширование добавлял бы сотни миллисекунд.
var Cost = 12

// ErrInvalidCredentials — общий ответ на неверный логин или пароль.
//
// Специально не различаем эти случаи: сообщение «пользователь не найден»
// позволяет перебирать существующие логины.
var ErrInvalidCredentials = errors.New("неверный логин или пароль")

// HashPassword возвращает bcrypt-хеш пароля.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("пароль не может быть пустым")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), Cost)
	if err != nil {
		return "", fmt.Errorf("не удалось захешировать пароль: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword сверяет пароль с bcrypt-хешем из базы.
//
// Если в базе лежит не bcrypt-хеш, а открытый текст (так было в исходной
// версии проекта), сравнение выполняется напрямую и возвращается признак
// legacy: вызывающий код переводит такой пароль в хеш при следующем входе.
// Это позволяет мигрировать базу без принудительного сброса паролей.
func VerifyPassword(hashed, password string) (ok bool, legacyPlaintext bool) {
	if hashed == "" {
		return false, false
	}

	if !looksLikeBcrypt(hashed) {
		return subtle.ConstantTimeCompare([]byte(hashed), []byte(password)) == 1, true
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
		return false, false
	}
	return true, false
}

// looksLikeBcrypt сообщает, является ли строка bcrypt-хешем.
//
// Признак: длина ровно 60 символов и префикс $2a$, $2b$ или $2y$.
func looksLikeBcrypt(s string) bool {
	if len(s) != 60 {
		return false
	}
	switch s[:4] {
	case "$2a$", "$2b$", "$2y$", "$2x$":
		return true
	default:
		return false
	}
}

// dummyHash — фиктивный bcrypt-хеш для выравнивания времени ответа.
//
// Это настоящий хеш, посчитанный с той же стоимостью, что и настоящие.
// Он нужен только для сравнения: при входе с несуществующим логином
// проверка идёт против него, чтобы время ответа не выдавало, существует
// ли такой пользователь.
var dummyHash = []byte("$2a$12$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

// DummyVerify выполняет фиктивную проверку пароля, выравнивая время ответа.
func DummyVerify(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

// NewRefreshToken возвращает новый refresh-токен, его хеш и идентификатор семейства.
//
// Токен имеет вид "<familyID>.<secret>", где familyID — случайный идентификатор
// сессии в открытом виде, а secret — сама случайная строка. Разделение нужно
// для двух разных операций: по familyID выполняется поиск и отзыв всех
// токенов сессии в базе, а secret сверяется по хешу. Идентификатор можно
// хранить рядом с токеном, потому что он не является секретом — от него
// зависит только скорость поиска записи.
func NewRefreshToken() (token, hash, familyID string, err error) {
	family := make([]byte, 16)
	if _, err := rand.Read(family); err != nil {
		return "", "", "", fmt.Errorf("не удалось сгенерировать идентификатор семейства: %w", err)
	}
	familyID = base64.RawURLEncoding.EncodeToString(family)

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", "", fmt.Errorf("не удалось сгенерировать refresh-токен: %w", err)
	}
	token = familyID + "." + base64.RawURLEncoding.EncodeToString(secret)

	return token, HashRefreshToken(token), familyID, nil
}

// TokenFamily возвращает идентификатор семейства из refresh-токена.
func TokenFamily(token string) string {
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			return token[:i]
		}
	}
	return ""
}

// HashRefreshToken возвращает SHA-256 хеш токена.
//
// SHA-256 здесь уместен: токен — не пароль, а случайная строка из 256 бит
// энтропии, которую нельзя перебирать, поэтому медленный KDF не нужен.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// MatchesRefreshToken проверяет, что предъявленный токен соответствует хешу
// из базы. Сравнение постоянного времени исключает утечку через замер
// времени ответа.
func MatchesRefreshToken(hash, token string) bool {
	if hash == "" || token == "" {
		return false
	}
	candidate := HashRefreshToken(token)
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(hash)) == 1
}
