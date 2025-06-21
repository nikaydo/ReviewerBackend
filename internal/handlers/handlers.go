package handles

import (
	"encoding/json"
	"fmt"
	"main/internal/ai"
	"main/internal/database"
	"main/internal/jwt"
	"main/internal/models"
	"main/internal/textanalize"
	"net/http"
	"time"
)

type Handlers struct {
	Pg database.Database
}

/*
	func (h *Handlers) ReqAi(w http.ResponseWriter, r *http.Request) {
		ask := r.FormValue("ask")
		stream, err := h.Ai.Requests(r.Context(), &pb.ReqRequest{Prompt: ask})
		if err != nil {
			fmt.Println(err)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Fprintf(w, "\n[error] %v\n", err)
				flusher.Flush()
				break
			}
			fmt.Fprintf(w, "%s\n", resp.Chunk)
			flusher.Flush()
		}
	}
*/

func (h *Handlers) ReviewAdd(w http.ResponseWriter, r *http.Request) {
	req := r.FormValue("req")
	model := r.FormValue("model")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	answer, err := ai.Generate(model, h.Pg.Env.EnvMap["MISTRAL_API_KEY"], req)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	err = h.Pg.Add(username, req, answer.Response, answer.Think, model)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ReviewDelete(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err := h.Pg.Delete(username, id); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)

}

func (h *Handlers) ReviewAnalize(w http.ResponseWriter, r *http.Request) {
	text := r.FormValue("text")
	resp, err := textanalize.Generate(text, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	writeJSONResponse(w, resp, http.StatusOK)
}

func (h *Handlers) ReviewGet(w http.ResponseWriter, r *http.Request) {
	_, username, err := GetUsername(w, r, h.Pg.Env)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	us, err := h.Pg.Get(username)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, us, http.StatusOK)
}

func (h *Handlers) SignIn(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	u, err := h.Pg.CheckUser(user.Login, user.Pass, true)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	j := jwt.JwtTokens{Env: h.Pg.Env}
	if err = j.CreateTokens(user.Id, user.Login, ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err = h.Pg.UpdateUser(u.Login, j.RefreshToken); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	http.SetCookie(w, MakeCookie("jwt", j.AccessToken, time.Duration(10*time.Minute)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(j.AccessToken))
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
}

func (h *Handlers) SignUp(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	n, err := h.Pg.CreateUser(user.Login, user.Pass)
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if n == 0 {
		writeErrorResponse(w, fmt.Errorf("ошибка"), http.StatusBadRequest)
		return
	}
	j := jwt.JwtTokens{Env: h.Pg.Env}
	if err = j.CreateTokens(user.Id, user.Login, ""); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	if err = h.Pg.UpdateUser(user.Login, j.RefreshToken); err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	http.SetCookie(w, MakeCookie("jwt", j.AccessToken, time.Duration(10*time.Minute)))
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(j.AccessToken))
	if err != nil {
		writeErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
