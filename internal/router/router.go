package router

import (
	"main/internal/database"
	h "main/internal/handlers"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

type Router struct {
	Handlers h.Handlers
}

func RouterInit(pg database.Database) Router {
	return Router{Handlers: h.Handlers{Pg: pg}}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rt *Router) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(withCORS)
	r.Route("/user", func(r chi.Router) {
		r.Get("/review/get", rt.Handlers.ReviewGet)
		r.Post("/review/add", rt.Handlers.ReviewAdd)
	})
	//r.Post("/ai/response", func(w http.ResponseWriter, r *http.Request) { rt.Handlers.ReqAi(w, r) })
	return r
}
