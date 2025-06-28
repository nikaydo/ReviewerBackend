package handles

import (
	"fmt"
	"net/http"
)

func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	model := r.FormValue("model")
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		if err := h.Pg.UpdateSettings(uuid, request, model); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	case http.MethodGet:
		u, err := h.Pg.GetSettings(uuid)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		writeJSONResponse(w, u, http.StatusOK)
	case http.MethodPost:
		if err := h.Pg.SaveSettings(uuid, "", ""); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}
