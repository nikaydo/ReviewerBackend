package handles

import (
	"fmt"
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
	uuid := r.FormValue("uuid")
	text := r.FormValue("text")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	h.Pg.UpdateReview(username, text, uuid)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) Favorite(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	favorite := r.FormValue("favorite")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewFavorite(username, favorite, uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewAdd(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	model := r.FormValue("model")
	up := r.FormValue("userpreset")
	uuid, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	mainPromt, preset, err := h.Pg.ReviewSum(uuid, up)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	answer, err := ai.Generate(model, h.Pg.Env.EnvMap["MISTRAL_API_KEY"], request, mainPromt+" Учти следующие уточнения "+preset, "", true, false)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	err = h.Pg.ReviewAdd(username, request, answer.Response, answer.Think, model)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewDelete(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewDelete(username, uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

}

func (h *Handlers) ReviewGenTitle(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	uuid := r.FormValue("uuid")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	tab, err := h.Pg.ReviewGetOne(username, uuid)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	answer, err := ai.Generate("ministral-3b-2410", h.Pg.Env.EnvMap["MISTRAL_API_KEY"], request, "", tab.Answer, false, true)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewTitleAdd(uuid, answer.Response, request); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewTitleUpdate(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	uuid := r.FormValue("uuid")
	if err := h.Pg.ReviewTitleUpdate(text, uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewTitleUpdateMainPromt(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewTitleUpdatePromt(text, uuid); err != nil {
		fmt.Println(err)
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewCustomPromtGet(w http.ResponseWriter, r *http.Request) {
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	ls, err := h.Pg.CustomPromtGet(uuid)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, ls, 200)
}
func (h *Handlers) ReviewCustomPromtAdd(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	promt := r.FormValue("promt")
	uuidUser, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.CustomPromtAdd(uuidUser, name, promt); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewCustomPromtDel(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	if err := h.Pg.CustomPromtDel(uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (h *Handlers) ReviewCustomPromtUpdate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	promt := r.FormValue("promt")
	uuid := r.FormValue("uuid")
	if err := h.Pg.CustomPromtUpdate(uuid, name, promt); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
