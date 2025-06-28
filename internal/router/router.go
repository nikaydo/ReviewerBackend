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
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", rt.Handlers.Settings)
			r.Put("/", rt.Handlers.Settings)
		})
		r.Route("/review", func(r chi.Router) {
			r.Route("/ask", func(r chi.Router) {
				r.Post("/", rt.Handlers.Ask)
				r.Put("/", rt.Handlers.Ask)
			})
			r.Post("/mainpromt", rt.Handlers.MainPrompt)
			r.Get("/", rt.Handlers.Review)
			r.Delete("/", rt.Handlers.Review)
			r.Post("/", rt.Handlers.Review)
			r.Put("/", rt.Handlers.Review)
			r.Post("/favorite", rt.Handlers.Favorite)
			r.Get("/brain", rt.Handlers.Memory)
			r.Post("/brain", rt.Handlers.Memory)
		})
		r.Route("/custom", func(r chi.Router) {
			r.Post("/", rt.Handlers.Custom)
			r.Get("/", rt.Handlers.Custom)
			r.Delete("/", rt.Handlers.Custom)
			r.Put("/", rt.Handlers.Custom)
		})
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "./web/user.html")
		})
	})
	r.Post("/signin", rt.Handlers.SignIn)
	r.Post("/signup", rt.Handlers.SignUp)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/index.html")
	})
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))
	return r
}
