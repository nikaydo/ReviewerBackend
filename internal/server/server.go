package server

import (
	"fmt"
	"main/internal/config"
	"main/internal/database"
	"main/internal/router"
	"net/http"
)

func ServerInit(pg database.Database, e config.Env) *http.Server {
	r := router.RouterInit(pg)
	return &http.Server{
		Addr:    fmt.Sprintf("%s:%s", e.EnvMap["HOST"], e.EnvMap["PORT"]),
		Handler: r.Router(),
	}
}
