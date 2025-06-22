package handles

import "net/http"

func (h *Handlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.SaveSettings(username, "", ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	request := r.FormValue("request")
	model := r.FormValue("model")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.UpdateSettings(username, request, model); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	u, err := h.Pg.GetSettings(username)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, u, http.StatusOK)
}
