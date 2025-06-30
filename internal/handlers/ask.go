package handles

import (
	"fmt"
	"main/internal/models"
	"main/internal/queue"
	"net/http"

	u "github.com/google/uuid"
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
		r := u.New()
		if err := h.Pg.InProgress(r, uuid_user); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		u, err := h.Pg.GetSettings(uuid_user)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		queue.AddInQueue(h.Queue, models.Enquiry{
			QueryUuid:   r,
			Uuid:        uuid_user,
			AskUuid:     uuid,
			Model:       "mistral-large-2411",
			Request:     request,
			Memory:      *u.Memory,
			System:      "",
			Assistant:   tab.Answer,
			IsSystem:    false,
			IsAssistant: true,
			Type:        2,
		})
		writeJSONResponse(w, models.ReturnUuid{UuidQuery: r}, http.StatusCreated)
		return
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
