package handles

import (
	"main/internal/jwt"
	"net/http"
	"strconv"
	"time"
)

func (h Handlers) CheckJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("jwt")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		_, username, err := jwt.ValidateToken(c.Value, h.Pg.Env.EnvMap["SECRET"])
		if err != nil {
			if err == jwt.ErrTokenExpired {
				user, err := h.Pg.CheckUser(username, "", false)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if _, _, err := jwt.ValidateToken(user.RefreshToken, h.Pg.Env.EnvMap["SECRET_REFRESH"]); err != nil {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				j := jwt.JwtTokens{Env: h.Pg.Env}
				if err = j.CreateTokens(user.Id, user.Login, ""); err != nil {
					writeErrorResponse(w, err, http.StatusBadRequest)
					return
				}
				if err := h.Pg.UpdateUser(user.Login, j.RefreshToken); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				cockie, err := strconv.Atoi(h.Pg.Env.EnvMap["COCKIE_TTL"])
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				http.SetCookie(w, MakeCookie("jwt", j.AccessToken, time.Duration(time.Duration(cockie)*time.Minute)))
				next.ServeHTTP(w, r)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
