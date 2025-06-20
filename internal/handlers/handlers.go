package handles

import (
	"fmt"
	"main/internal/ai"
	"main/internal/database"
	"net/http"
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
	answer, think := ai.Generate("magistral-medium-2506", "", req)
	err := h.Pg.Add("test", req, answer, think)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (h *Handlers) ReviewGet(w http.ResponseWriter, r *http.Request) {
	us := h.Pg.Get("test")
	writeJSONResponse(w, us, http.StatusOK)
}
