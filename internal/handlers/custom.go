package handles

import (
	"fmt"
	"net/http"
)

func (h *Handlers) Custom(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	promt := r.FormValue("promt")
	uuid := r.FormValue("uuid")
	uuidUser, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		ls, err := h.Pg.CustomPromtGet(uuidUser)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		writeJSONResponse(w, ls, 200)
	case http.MethodPost:
		if err := h.Pg.CustomPromtAdd(uuidUser, name, promt); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	case http.MethodPut:
		if err := h.Pg.CustomPromtUpdate(uuid, name, promt); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	case http.MethodDelete:
		if err := h.Pg.CustomPromtDel(uuid); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}
