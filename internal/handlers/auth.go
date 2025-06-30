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
	err = tokensAndCookie(w, h, user, u.Uuid)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
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
	err = tokensAndCookie(w, h, user, uuid)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func tokensAndCookie(w http.ResponseWriter, h *Handlers, user models.User, uuid string) error {
	if err := h.Pg.Remember(uuid, ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return err
	}
	j := jwt.JwtTokens{Env: h.Pg.Env}
	if err := j.CreateTokens(uuid, user.Login, ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return err
	}
	if err := h.Pg.UpdateUser(user.Login, j.RefreshToken); err != nil {
		return err
	}
	cockie, err := strconv.Atoi(h.Pg.Env.EnvMap["COCKIE_TTL"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.SetCookie(w, MakeCookie("jwt", j.AccessToken, time.Duration(time.Duration(cockie)*time.Minute)))
	return nil
}
