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
		r.Route("/setting", func(r chi.Router) {
			r.Get("/get", rt.Handlers.GetSettings)
			r.Get("/save", rt.Handlers.SaveSettings)
			r.Post("/update", rt.Handlers.UpdateSettings)
		})
		r.Route("/review", func(r chi.Router) {
			r.Route("/title", func(r chi.Router) {
				r.Post("/add", rt.Handlers.ReviewGenTitle)
				r.Post("/update", rt.Handlers.ReviewTitleUpdate)
			})
			r.Get("/get", rt.Handlers.ReviewGet)
			r.Post("/delete", rt.Handlers.ReviewDelete)
			r.Post("/add", rt.Handlers.ReviewAdd)
			r.Post("/update", rt.Handlers.ReviewUpdate)
			r.Post("/favorite/set", rt.Handlers.Favorite)
		})
	})
	r.Post("/signin", rt.Handlers.SignIn)
	r.Post("/signup", rt.Handlers.SignUp)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))
	return r
}
