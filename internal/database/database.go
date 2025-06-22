package database

import (
	"context"
	"log"
	"main/internal/config"

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
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS rewusers (
		id SERIAL PRIMARY KEY,
		login TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		refresh_token TEXT NOT NULL
		);`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS reviewTears (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			request TEXT NOT NULL,
			answer TEXT,
			date TIMESTAMPTZ,
			model TEXT,
			favorite BOOLEAN,
			think TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS userSetting (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			request TEXT,
			model TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	return Database{Pg: conn, Env: e}
}
