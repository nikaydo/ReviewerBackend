package handles

import (
	"fmt"
	"main/internal/models"
	"net/http"
)

func (h *Handlers) Memory(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		str, err := h.Pg.Recall(uuid_user)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		writeJSONResponse(w, models.Memory{Mem: str}, http.StatusOK)
	case http.MethodPost:
		if err := h.Pg.KeepInMind(uuid_user, text); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}
