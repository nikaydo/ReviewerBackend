package handles

import "net/http"

func (h *Handlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.SaveSettings(uuid, "", ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	model := r.FormValue("model")
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.UpdateSettings(uuid, request, model); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	uuid, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	u, err := h.Pg.GetSettings(uuid)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, u, http.StatusOK)
}
