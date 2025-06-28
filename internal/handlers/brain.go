package handles

import (
	"fmt"
	"net/http"
)

func (h *Handlers) Memory(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	uuid := r.FormValue("uuid")
	switch r.Method {
	case http.MethodGet:
		str, err := h.Pg.Recall(uuid)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		fmt.Println(str)
	case http.MethodPost:
		if err := h.Pg.Remember(uuid, text); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}
