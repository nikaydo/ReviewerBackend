package server

import (
	"fmt"
	"main/internal/config"
	"main/internal/database"
	"main/internal/models"
	"main/internal/router"
	"net/http"
)

func ServerInit(pg database.Database, e config.Env, q *models.List) *http.Server {
	r := router.RouterInit(pg, q)
	return &http.Server{
		Addr:    fmt.Sprintf("%s:%s", e.EnvMap["HOST"], e.EnvMap["PORT"]),
		Handler: r.Router(),
	}
}
