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

func (rt *Router) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/user", func(r chi.Router) {
		r.Use(rt.Handlers.CheckJWT)
		r.Get("/review/get", rt.Handlers.ReviewGet)
		r.Post("/review/delete", rt.Handlers.ReviewDelete)
		r.Post("/review/text/analyze", rt.Handlers.ReviewAnalize)
		r.Post("/review/add", rt.Handlers.ReviewAdd)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "./web/index.html")
		})
	})
	r.Post("/signin", rt.Handlers.SignIn)
	r.Post("/signup", rt.Handlers.SignUp)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/hello.html")
	})
	r.Get("/signin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/auth.html")
	})
	r.Handle("/bg/*", http.StripPrefix("/bg/", http.FileServer(http.Dir("./web/bg"))))

	//r.Post("/ai/response", func(w http.ResponseWriter, r *http.Request) { rt.Handlers.ReqAi(w, r) })
	return r
}
