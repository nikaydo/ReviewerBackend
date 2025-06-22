package handles

import (
	"main/internal/ai"
	"net/http"
)

func (h *Handlers) ReviewGet(w http.ResponseWriter, r *http.Request) {
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	us, err := h.Pg.ReviewGet(username)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, us, http.StatusOK)
}

func (h *Handlers) ReviewUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	text := r.FormValue("text")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	h.Pg.UpdateReview(username, text, id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) Favorite(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	favorite := r.FormValue("favorite")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewFavorite(username, favorite, id); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewAdd(w http.ResponseWriter, r *http.Request) {
	req := r.FormValue("req")
	model := r.FormValue("model")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	answer, err := ai.Generate(model, h.Pg.Env.EnvMap["MISTRAL_API_KEY"], req, "", "")
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	err = h.Pg.ReviewAdd(username, req, answer.Response, answer.Think, model)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewDelete(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewDelete(username, id); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

}
