package handles

import (
	"main/internal/queue"
	"net/http"

	u "github.com/google/uuid"
)

func (h *Handlers) QueueGet(w http.ResponseWriter, r *http.Request) {
	uuidString := r.FormValue("uuid")
	uuid, err := u.Parse(uuidString)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	Me := queue.WhereIAm(h.Queue, uuid)
	writeJSONResponse(w, Me, http.StatusOK)
}
