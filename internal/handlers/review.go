package handles

import (
	"fmt"
	"main/internal/ai"
	"net/http"
	"strings"
)

func (h *Handlers) Review(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	text := r.FormValue("text")
	request := r.FormValue("request")
	model := r.FormValue("model")
	up := r.FormValue("userpreset")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		us, err := h.Pg.ReviewGet(uuid_user)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		writeJSONResponse(w, us, http.StatusOK)
	case http.MethodDelete:
		if err := h.Pg.ReviewDelete(uuid_user, uuid); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	case http.MethodPost:
		mainPromt, preset, err := h.Pg.ReviewSum(uuid_user, up)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		mem, err := h.Pg.Recall(uuid_user)
		if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		answer, err := ai.Generate(model, h.Pg.Env.EnvMap["MISTRAL_API_KEY"], request, makePrompt(mainPromt, mem, preset), "", true, false)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		res := strings.ReplaceAll(answer.Response, "—", "-")
		if err = h.Pg.ReviewAdd(uuid_user, request, res, answer.Think, model); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	case http.MethodPut:
		mem, err := h.Pg.Recall(uuid_user)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		s := strings.Replace(h.Pg.Env.EnvMap["PROMPT_FOR_MEMORIZATION"], "<memory>", mem, 1)
		memoryClean, err := ai.Generate("mistral-large-2411", h.Pg.Env.EnvMap["MISTRAL_API_KEY"], request, s, "", true, false)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		fmt.Println(s, memoryClean.Response)
		err = h.Pg.KeepInMind(uuid_user, memoryClean.Response)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		if err := h.Pg.UpdateReview(uuid_user, text, uuid); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) Favorite(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	favorite := r.FormValue("favorite")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewFavorite(uuid_user, favorite, uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) MainPrompt(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewTitleUpdatePromt(text, uuid_user); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
