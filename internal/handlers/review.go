package handles

import (
	"main/internal/ai"
	"net/http"
)

func (h *Handlers) ReviewGet(w http.ResponseWriter, r *http.Request) {
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	us, err := h.Pg.ReviewGet(uuid_user)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, us, http.StatusOK)
}

func (h *Handlers) ReviewUpdate(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	text := r.FormValue("text")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	h.Pg.UpdateReview(uuid_user, text, uuid)
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

func (h *Handlers) ReviewAdd(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	model := r.FormValue("model")
	up := r.FormValue("userpreset")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	mainPromt, preset, err := h.Pg.ReviewSum(uuid_user, up)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	answer, err := ai.Generate(model, h.Pg.Env.EnvMap["MISTRAL_API_KEY"], request, mainPromt+" Учти следующие уточнения "+preset, "", true, false)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	err = h.Pg.ReviewAdd(uuid_user, request, answer.Response, answer.Think, model)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewDelete(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewDelete(uuid_user, uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

}

func (h *Handlers) ReviewGenTitle(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	uuid := r.FormValue("uuid")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	tab, err := h.Pg.ReviewGetOne(uuid_user, uuid)
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
