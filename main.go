package main

import (
	"fmt"
	"main/internal/database"
	"main/internal/server"
	"net/http"
)

func main() {
	pg := database.InitBD()
	server := server.ServerInit(pg)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
	}

}
