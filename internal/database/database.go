package database

import (
	"context"
	"log"
	"main/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
)

type Database struct {
	Pg  *pgx.Conn
	Env config.Env
}

func InitBD(e config.Env) Database {
	conn, err := pgx.Connect(context.Background(), e.EnvMap["POSTGRESS_ADDR"])
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	return Database{Pg: conn, Env: e}
}

func RunMigrations(e config.Env) {
	m, err := migrate.New(
		"file://db/migrations",
		e.EnvMap["POSTGRESS_ADDR"],
	)
	if err != nil {
		log.Fatal("ошибка инициализации миграций:", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal("ошибка применения миграций:", err)
	}

	log.Println("миграции успешно применены")
}
