package handles

import (
	"encoding/json"
	"main/internal/jwt"
	"main/internal/models"
	"net/http"
	"strconv"
	"time"
)

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	u, err := h.Pg.CheckUser(user.Login, user.Pass, true)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	j := jwt.JwtTokens{Env: h.Pg.Env}
	if err = j.CreateTokens(u.Uuid, u.Login, ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err = h.Pg.UpdateUser(u.Login, j.RefreshToken); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	cockie, err := strconv.Atoi(h.Pg.Env.EnvMap["COCKIE_TTL"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.SetCookie(w, MakeCookie("jwt", j.AccessToken, time.Duration(time.Duration(cockie)*time.Minute)))
	w.WriteHeader(http.StatusOK)

}

func (h *Handlers) SignUp(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	uuid, err := h.Pg.CreateUser(user.Login, user.Pass)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	j := jwt.JwtTokens{Env: h.Pg.Env}
	if err = j.CreateTokens(uuid, user.Login, ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err = h.Pg.UpdateUser(user.Login, j.RefreshToken); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	cockie, err := strconv.Atoi(h.Pg.Env.EnvMap["COCKIE_TTL"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.SetCookie(w, MakeCookie("jwt", j.AccessToken, time.Duration(time.Duration(cockie)*time.Minute)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(j.AccessToken))
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
