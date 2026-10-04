package handlers

import (
	"errors"
	"net/http"

	"github.com/nikaydo/reviewer/internal/auth"
	"github.com/nikaydo/reviewer/internal/database"
	"github.com/nikaydo/reviewer/internal/jwt"
)

// credentials — тело запроса регистрации и входа.
type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// userResponse — публичные сведения о пользователе.
type userResponse struct {
	ID    string `json:"uuid"`
	Login string `json:"login"`
	Role  string `json:"role"`
}

// SignUp регистрирует нового пользователя.
//
// Раньше регистрация сразу выдавала сессию, поэтому любой мог завести
// тысячу аккаунтов. Теперь на входе стоит ограничение частоты запросов,
// а пароль сразу сохраняется в виде bcrypt-хеша.
func (h *Handlers) SignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var creds credentials
	if err := decodeJSON(w, r, &creds, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}

	creds.Login = database.NormalizeLogin(creds.Login)
	if err := database.ValidateLogin(creds.Login); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "invalid_login", err.Error(), err)
		return
	}
	if err := database.ValidatePassword(creds.Password); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "weak_password", err.Error(), err)
		return
	}

	hash, err := auth.HashPassword(creds.Password)
	if err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}

	userID, err := h.Store.CreateUser(ctx, database.CreateUserParams{
		Login:         creds.Login,
		PasswordHash:  hash,
		Role:          jwt.RoleViewer,
		DefaultPrompt: h.Cfg.DefaultMainPrompt,
		MemoryPrompt:  h.Cfg.MemorizationPrompt,
	})
	if err != nil {
		if errors.Is(err, database.ErrLoginTaken) {
			fail(ctx, w, h.Log, http.StatusConflict, "login_taken", "логин уже занят")
			return
		}
		failErr(ctx, w, h.Log, http.StatusBadRequest, "signup_failed", "не удалось зарегистрироваться", err)
		return
	}

	// Пустая запись памяти, чтобы её не приходилось создавать лениво
	// посреди обработки запроса.
	if err := h.Store.CreateMemory(ctx, userID); err != nil {
		h.Log.Error("не удалось создать запись памяти", "user", userID, "err", err)
	}

	if err := h.issueSession(ctx, w, userID, creds.Login, jwt.RoleViewer, "", ""); err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "не удалось создать сессию", err)
		return
	}

	writeJSON(w, http.StatusCreated, userResponse{ID: userID, Login: creds.Login, Role: jwt.RoleViewer})
}

// SignIn проверяет пароль и выдаёт сессию.
//
// Ответ одинаков для несуществующего пользователя и неверного пароля —
// иначе по сообщению можно перебрать существующие логины. Чтобы не давать
// дополнительный сигнал и по времени ответа, при отсутствии пользователя
// выполняется фиктивная проверка bcrypt.
func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var creds credentials
	if err := decodeJSON(w, r, &creds, h.Cfg.MaxRequestBytes); err != nil {
		failErr(ctx, w, h.Log, http.StatusBadRequest, "bad_request", "не удалось прочитать тело запроса", err)
		return
	}

	user, err := h.Store.UserByLogin(ctx, creds.Login)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			// Считаем пароль против фиктивного хеша: bcrypt дорогой, и без
			// этого запрос с несуществующим логином возвращался бы заметно
			// быстрее, что тоже раскрывает список пользователей.
			auth.DummyVerify(creds.Password)
			fail(ctx, w, h.Log, http.StatusUnauthorized, "invalid_credentials", "неверный логин или пароль")
			return
		}
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}

	ok, legacy := auth.VerifyPassword(user.PasswordHash, creds.Password)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "invalid_credentials", "неверный логин или пароль")
		return
	}

	// Миграция со старого формата: пароль в базе лежал открытым текстом.
	// Теперь хешируем его и отзываем все сессии, чтобы старые токены,
	// выпущенные до смены пароля, перестали работать.
	if legacy {
		hash, err := auth.HashPassword(creds.Password)
		if err != nil {
			failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
			return
		}
		if err := h.Store.UpdatePasswordHash(ctx, user.ID, hash); err != nil {
			h.Log.Error("не удалось обновить хеш пароля", "user", user.ID, "err", err)
		} else {
			h.Log.Info("пароль переведён на bcrypt", "user", user.ID)
			if err := h.Store.RevokeAllUserTokens(ctx, user.ID); err != nil {
				h.Log.Error("не удалось отозвать прошлые сессии", "user", user.ID, "err", err)
			}
		}
	}

	if err := h.issueSession(ctx, w, user.ID, user.Login, user.Role, "", ""); err != nil {
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "не удалось создать сессию", err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse{ID: user.ID, Login: user.Login, Role: user.Role})
}

// Refresh обновляет сессию по refresh-токену.
//
// Ротация: предъявленный токен заменяется новым, а старый становится
// недействительным. Если пришёл уже отозванный токен, значит он был украден,
// и отзывается всё семейство сессии.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_refresh_token", "отсутствует refresh-токен")
		return
	}

	presented := cookie.Value
	familyID := auth.TokenFamily(presented)
	if familyID == "" {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "refresh_invalid", "refresh-токен недействителен")
		return
	}

	presentedHash := auth.HashRefreshToken(presented)
	user, err := h.Store.UserByRefreshHash(ctx, presentedHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			fail(ctx, w, h.Log, http.StatusUnauthorized, "refresh_invalid", "refresh-токен недействителен")
			return
		}
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "внутренняя ошибка сервера", err)
		return
	}

	if err := h.issueSession(ctx, w, user.ID, user.Login, user.Role, familyID, presentedHash); err != nil {
		var reused *database.ErrRefreshReused
		if errors.As(err, &reused) {
			// Токен уже использовали: считаем сессию скомпрометированной.
			if revokeErr := h.Store.RevokeTokenFamily(ctx, familyID); revokeErr != nil {
				h.Log.Error("не удалось отозвать семейство токенов", "family", familyID, "err", revokeErr)
			}
			h.clearAuthCookies(w)
			h.Log.Warn("повторное использование refresh-токена",
				"user", user.ID, "family", familyID)
			fail(ctx, w, h.Log, http.StatusUnauthorized, "refresh_reused", "сессия завершена")
			return
		}
		failErr(ctx, w, h.Log, http.StatusInternalServerError, "internal", "не удалось обновить сессию", err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse{ID: user.ID, Login: user.Login, Role: user.Role})
}

// Logout завершает сессию.
//
// Токен отзывается в базе, а не просто удаляется из cookie: иначе украденный
// refresh-токен продолжал бы работать до истечения срока.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if cookie, err := r.Cookie(refreshCookieName); err == nil && cookie.Value != "" {
		hash := auth.HashRefreshToken(cookie.Value)
		if user, err := h.Store.UserByRefreshHash(ctx, hash); err == nil {
			if err := h.Store.RevokeRefreshToken(ctx, user.ID, hash); err != nil {
				h.Log.Error("не удалось отозвать refresh-токен", "user", user.ID, "err", err)
			}
		}
	}

	h.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me возвращает сведения о текущем пользователе.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s, ok := sessionFrom(ctx)
	if !ok {
		fail(ctx, w, h.Log, http.StatusUnauthorized, "no_session", "требуется авторизация")
		return
	}
	writeJSON(w, http.StatusOK, userResponse{ID: s.UserID, Login: s.Login, Role: s.Role})
}
