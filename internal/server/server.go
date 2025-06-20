package server

import (
	"main/internal/database"
	"main/internal/router"
	"net/http"
)

func ServerInit(pg database.Database) *http.Server {
	r := router.RouterInit(pg)
	return &http.Server{
		Addr:    "localhost:8080",
		Handler: r.Router(),
	}
}
