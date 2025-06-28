package main

import (
	"fmt"
	"log"
	"main/internal/config"
	"main/internal/database"
	"main/internal/server"
	"net/http"
)

func main() {
	env, err := config.ReadEnv()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}
	database.RunMigrations(env)
	pg := database.InitBD(env)
	server := server.ServerInit(pg, env)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println(err)
	}
}
