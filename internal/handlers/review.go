package handles

import (
	"fmt"
	"main/internal/models"
	"main/internal/queue"
	"net/http"

	u "github.com/google/uuid"
)

func (h *Handlers) Review(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	text := r.FormValue("text")
	request := r.FormValue("request")
	model := r.FormValue("model")
	up := r.FormValue("userpreset")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		us, err := h.Pg.ReviewGet(uuid_user)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		writeJSONResponse(w, us, http.StatusOK)
	case http.MethodDelete:
		if err := h.Pg.ReviewDelete(uuid_user, uuid); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	case http.MethodPost:
		mainPromt, preset, err := h.Pg.ReviewSum(uuid_user, up)
		if err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		mem, err := h.Pg.Recall(uuid_user)
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
		syst := ""
		if *u.Memory {
			syst = makePrompt(mainPromt, mem, preset)
		}
		queue.AddInQueue(h.Queue, models.Enquiry{
			QueryUuid:   r,
			Uuid:        uuid_user,
			Model:       model,
			Request:     request,
			System:      syst,
			Memory:      *u.Memory,
			Assistant:   "",
			IsSystem:    true,
			IsAssistant: false,
			Type:        1,
		})
		writeJSONResponse(w, models.ReturnUuid{UuidQuery: r}, http.StatusCreated)
		return
	case http.MethodPut:
		uuid = r.FormValue("uuid")
		if err := h.Pg.UpdateReview(uuid_user, text, uuid); err != nil {
			writeErrorResponse(w, err, http.StatusBadRequest)
			return
		}
	default:
		writeErrorResponse(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) Favorite(w http.ResponseWriter, r *http.Request) {
	uuid := r.FormValue("uuid")
	favorite := r.FormValue("favorite")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewFavorite(uuid_user, favorite, uuid); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) MainPrompt(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	uuid_user, _, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.ReviewTitleUpdatePromt(text, uuid_user); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
