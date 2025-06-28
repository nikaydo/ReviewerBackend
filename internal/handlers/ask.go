package handles

import (
	"fmt"
	"main/internal/ai"
	"net/http"
)

func (h *Handlers) Ask(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	uuid := r.FormValue("uuid")
	text := r.FormValue("text")
	switch r.Method {
	case http.MethodPost:
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
	case http.MethodPut:
		if err := h.Pg.ReviewTitleUpdate(text, uuid); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}
