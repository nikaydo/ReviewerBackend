package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5"
)

type Database struct {
	Pg *pgx.Conn
}

func InitBD() Database {
	conn, err := pgx.Connect(context.Background(), "postgres://myuser:mypassword@localhost:5432/mydb")
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}

	_, err = conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS reviews (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			request TEXT NOT NULL,
			answer TEXT,
			think TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}

	return Database{Pg: conn}
}

func (d *Database) Add(user, request, answer, think string) error {
	_, err := d.Pg.Exec(context.Background(), "INSERT INTO reviews (username, request,answer,think) VALUES ($1, $2, $3, $4)", user, request, answer, think)
	if err != nil {
		return err
	}
	return nil
}

type UserTab struct {
	User    string `json:"user"`
	Request string `json:"request"`
	Answer  string `json:"answer"`
	Think   string `json:"think"`
}

func (d *Database) Get(user string) []UserTab {
	rows, err := d.Pg.Query(context.Background(), "SELECT request,answer,think FROM reviews WHERE username = $1", user)
	if err != nil {
		log.Fatalf("Ошибка выборки: %v", err)
	}
	defer rows.Close()
	var ls []UserTab
	for rows.Next() {
		var u UserTab
		if err := rows.Scan(&u.Request, &u.Answer, &u.Think); err != nil {
			log.Fatalf("Ошибка чтения строки: %v", err)
		}
		ls = append(ls, u)
	}
	return ls
}
