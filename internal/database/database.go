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
	_, err = conn.Exec(context.Background(), `CREATE EXTENSION IF NOT EXISTS pgcrypto;`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS `+e.EnvMap["DB_USER"]+` (
		uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		login TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		refresh_token TEXT NOT NULL
		);`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS `+e.EnvMap["DB_REVIEW"]+` (
			uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
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
		CREATE TABLE IF NOT EXISTS `+e.EnvMap["DB_REVIEW_ASK"]+` (
			uuidReview UUID PRIMARY KEY,
			title TEXT,
			request TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS `+e.EnvMap["DB_USER_SETTING"]+` (
			uuid UUID NOT NULL,
			request TEXT,
			mainPromt TEXT,
			model TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS `+e.EnvMap["DB_USER_PROMT"]+` (
			uuidUser UUID,
			uuidUniq UUID DEFAULT gen_random_uuid(),
			name TEXT,
			promt TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}
	return Database{Pg: conn, Env: e}
}
