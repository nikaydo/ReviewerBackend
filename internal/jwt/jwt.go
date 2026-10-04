// Package jwt выпускает и проверяет токены доступа.
//
// Access-токен короткоживущий и хранится в cookie, refresh-токен живёт
// долго и выдаётся отдельно. Оба подписываются алгоритмом HMAC-SHA256,
// алгоритм проверяется явно — иначе на алгоритм «none» или на подмену ключа
// подвержены подделыванию.
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Роли пользователей.
const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

// Ошибки проверки токена. Их различает middleware: по ErrTokenExpired
// запускается ротация, остальные ошибки означают отказ.
var (
	ErrTokenExpired   = errors.New("срок действия токена истёк")
	ErrTokenInvalid   = errors.New("токен недействителен")
	ErrTokenMalformed = errors.New("токен повреждён")
	ErrUnexpectedAlg  = errors.New("неожиданный алгоритм подписи")
)

// Claims — содержимое access-токена.
type Claims struct {
	jwt.RegisteredClaims
	Login string `json:"login"`
	Role  string `json:"role"`
}

// Manager выпускает и проверяет токены.
type Manager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
	now       func() time.Time
}

// NewManager создаёт менеджер токенов.
func NewManager(secret, issuer string, accessTTL time.Duration) *Manager {
	return &Manager{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
		now:       time.Now,
	}
}

// NewAccessToken выпускает access-токен для пользователя.
func (m *Manager) NewAccessToken(userID, login, role string) (string, time.Time, error) {
	issuedAt := m.now()
	expiresAt := issuedAt.Add(m.accessTTL)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.issuer},
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        fmt.Sprintf("%d", issuedAt.UnixNano()),
		},
		Login: login,
		Role:  role,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("не удалось подписать токен: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse проверяет access-токен и возвращает его claims.
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}

	parsed, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			// Проверяем именно тот алгоритм, которым подписан токен.
			// Без этой проверки библиотека примет и HS512, и «none».
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrUnexpectedAlg
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		// Проверка срока действия идёт по now из Manager, а не по
		// системным часам библиотеки: так время можно подменять в тестах
		// и контролировать в коде.
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired), errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, ErrTokenExpired
		case errors.Is(err, ErrUnexpectedAlg):
			return nil, ErrUnexpectedAlg
		case errors.Is(err, jwt.ErrTokenMalformed), errors.Is(err, jwt.ErrSignatureInvalid):
			return nil, ErrTokenInvalid
		default:
			return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
		}
	}
	if !parsed.Valid {
		return nil, ErrTokenInvalid
	}
	if claims.Subject == "" {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}
