package handles

import (
	"fmt"
	"net/http"
	"strconv"
)

func (h *Handlers) Settings(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	model := r.FormValue("model")
	memory := r.FormValue("memory")
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		sameTime, _ := strconv.Atoi(h.Pg.Env.EnvMap["QUEUE_SAME_TIME_PROCESSED"])
		if err := h.Pg.UpdateSettings(uuid, request, model, memory, sameTime); err != nil {
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
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}
