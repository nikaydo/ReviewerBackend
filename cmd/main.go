package main

import (
	"log"
	"main/internal/config"
	"main/internal/database"
	"main/internal/models"
	"main/internal/queue"
	"main/internal/server"
	"net/http"
)

func main() {
	var Query *models.List = &models.List{Request: []models.Enquiry{}}
	env, err := config.ReadEnv()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}
	if err := database.RunMigrations(env); err != nil {
		log.Fatalf("Migrations not accepted: %v", err)
	}
	pg := database.InitBD(env)
	go queue.RunQueue(Query, env, pg)
	server := server.ServerInit(pg, env, Query)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Cant starting server: %v", err)
	}
	/*
	   stop := make(chan os.Signal, 1)
	   signal.Notify(stop, os.Interrupt)

	   server := server.ServerInit(pg, env, Query)

	   	go func() {
	   		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	   			log.Fatalf("Cant starting server: %v", err)
	   		}
	   	}()

	   <-stop
	   log.Println("Shutting down server...")
	   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	   defer cancel()

	   	if err := server.Shutdown(ctx); err != nil {
	   		log.Fatalf("Server forced to shutdown: %v", err)
	   	}

	   log.Println("Server gracefully stopped")
	*/
}
