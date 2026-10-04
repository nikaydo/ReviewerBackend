package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nikaydo/reviewer/internal/auth"
	"github.com/nikaydo/reviewer/internal/jwt"
)

// cookieName — имя cookie с access-токеном.
const (
	// refreshCookieName — имя cookie с refresh-токеном.
	//
	// Хранить refresh-токен только в HttpOnly-cookie безопаснее, чем
	// возвращать его в теле ответа: тогда он недоступен JavaScript, а значит
	// не может быть украден через XSS.
	refreshCookieName = "refresh"
)

// Authenticate проверяет access-токен и кладёт данные сессии в контекст.
//
// Отдельный маршрут /user/refresh не проходит через эту проверку: на нём
// access-токен уже истёк, иначе ротация была бы недостижимой.
func (h *Handlers) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cookie, err := r.Cookie(h.Cfg.CookieName)
		if err != nil {
			fail(ctx, w, h.Log, http.StatusUnauthorized, "no_token", "требуется авторизация")
			return
		}

		claims, err := h.Tokens.Parse(cookie.Value)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				// Истёкший access-токен — не ошибка доступа, а сигнал
				// клиенту обменять его на новый.
				w.Header().Set("X-Auth-Reason", "expired")
				fail(ctx, w, h.Log, http.StatusUnauthorized, "token_expired", "срок действия токена истёк")
			default:
				fail(ctx, w, h.Log, http.StatusUnauthorized, "token_invalid", "токен недействителен")
			}
			return
		}

		s := session{
			UserID: claims.Subject,
			Login:  claims.Login,
			Role:   claims.Role,
		}
		// Refresh-токен нужен для ротации и хранится в отдельной cookie.
		if rc, err := r.Cookie(refreshCookieName); err == nil {
			s.RefreshHash = auth.HashRefreshToken(rc.Value)
			s.FamilyID = auth.TokenFamily(rc.Value)
		}

		next.ServeHTTP(w, r.WithContext(withSession(ctx, s)))
	})
}

// issueSession выдаёт новую пару токенов и устанавливает cookie.
//
// Если задан rotateFrom, предыдущий токен семейства заменяется новым —
// это ротация при обновлении сессии. Иначе создаётся новое семейство.
func (h *Handlers) issueSession(ctx context.Context, w http.ResponseWriter, userID, login, role, familyID, rotateFrom string) error {
	accessToken, accessExp, err := h.Tokens.NewAccessToken(userID, login, role)
	if err != nil {
		return err
	}

	refreshToken, refreshHash, newFamily, err := auth.NewRefreshToken()
	if err != nil {
		return err
	}

	refreshExp := time.Now().Add(h.Cfg.RefreshTokenTTL)

	if rotateFrom != "" {
		// Ротация: предыдущий токен семейства заменяется новым.
		if err := h.Store.RotateRefreshToken(ctx, userID, familyID, rotateFrom, refreshHash, refreshExp); err != nil {
			return err
		}
	} else {
		if err := h.Store.SaveRefreshToken(ctx, userID, newFamily, refreshHash, refreshExp); err != nil {
			return err
		}
	}

	h.setAuthCookies(w, accessToken, accessExp, refreshToken, refreshExp)
	return nil
}

// setAuthCookies устанавливает cookie с токенами.
func (h *Handlers) setAuthCookies(w http.ResponseWriter, accessToken string, accessExp time.Time, refreshToken string, refreshExp time.Time) {
	http.SetCookie(w, h.buildCookie(h.Cfg.CookieName, accessToken, accessExp))
	http.SetCookie(w, h.buildCookie(refreshCookieName, refreshToken, refreshExp))
}

// buildCookie собирает cookie с безопасными флагами.
func (h *Handlers) buildCookie(name, value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.Cfg.CookieSecure,
		// SameSite=Strict не даёт отправить cookie на сайт, инициировавший
		// переход: это закрывает CSRF через межсайтовые формы.
		SameSite: http.SameSiteStrictMode,
	}
}

// clearAuthCookies удаляет cookie с токенами.
func (h *Handlers) clearAuthCookies(w http.ResponseWriter) {
	expired := &http.Cookie{
		Name:     h.Cfg.CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.Cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, expired)
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.Cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}
