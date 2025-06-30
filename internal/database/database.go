package database

import (
	"context"
	"fmt"
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

func RunMigrations(e config.Env) error {
	m, err := migrate.New(
		"file://db/migrations",
		e.EnvMap["POSTGRESS_ADDR"],
	)
	if err != nil {
		return fmt.Errorf("ошибка инициализации миграций: %w", err)
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil || dbErr != nil {
			log.Printf("ошибка закрытия миграций: sourceErr=%v, dbErr=%v", sourceErr, dbErr)
		}
	}()

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {

		return fmt.Errorf("ошибка применения миграций: %w", err)
	}

	log.Println("миграции успешно применены")
	return nil
}
