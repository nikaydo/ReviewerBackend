package database

import (
	"context"
	"log"
	"main/internal/config"
	"main/internal/models"
	"time"

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
		CREATE TABLE IF NOT EXISTS reviewT (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			request TEXT NOT NULL,
			answer TEXT,
			date TIMESTAMPTZ,
			model TEXT,
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

func (d *Database) Add(user, request, answer, think, model string) error {
	_, err := d.Pg.Exec(context.Background(), "INSERT INTO reviewT (username, request,answer,think,date,model) VALUES ($1, $2, $3, $4, $5,$6)", user, request, answer, think, time.Now(), model)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) Get(user string) ([]models.UserTab, error) {
	rows, err := d.Pg.Query(context.Background(), "SELECT id,request,answer,think,date,model FROM reviewT WHERE username = $1", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ls []models.UserTab
	for rows.Next() {
		var u models.UserTab
		if err := rows.Scan(&u.Id, &u.Request, &u.Answer, &u.Think, &u.Date, &u.Model); err != nil {
			log.Fatalf("Ошибка чтения строки: %v", err)
		}
		ls = append(ls, u)
	}
	return ls, nil
}

func (d *Database) GetOne(user, id string) (models.UserTab, error) {
	rows := d.Pg.QueryRow(context.Background(), "SELECT id,request,answer,think,date,model FROM reviewT WHERE username = $1 AND id = $2", user, id)

	var u models.UserTab
	if err := rows.Scan(&u.Id, &u.Request, &u.Answer, &u.Think, &u.Date, &u.Model); err != nil {
		log.Fatalf("Ошибка чтения строки: %v", err)
	}
	return u, nil
}

func (d *Database) Delete(user, id string) error {
	_, err := d.Pg.Exec(context.Background(), "DELETE FROM reviewT WHERE username = $1 AND id = $2", user, id)
	if err != nil {
		return err
	}
	return nil
}

func (u *Database) CreateUser(Login, Pass string) (int64, error) {
	_, err := u.Pg.Exec(context.Background(), `
		INSERT INTO rewusers (login,password,refresh_token)
		VALUES ($1,$2,$3);`, Login, Pass, "")
	if err != nil {
		return 0, err
	}
	return int64(1), nil
}

func (u *Database) CheckUser(Login, Pass string, pass bool) (models.User, error) {
	var err error
	var user models.User
	if pass {
		err = u.Pg.QueryRow(context.Background(), `SELECT * FROM rewusers WHERE login = $1 AND password = $2;`, Login, Pass).Scan(&user.Id, &user.Login, &user.Pass, &user.RefreshToken)
	} else {
		err = u.Pg.QueryRow(context.Background(), `SELECT * FROM rewusers WHERE login = $1;`, Login).Scan(&user.Id, &user.Login, &user.Pass, &user.RefreshToken)
	}
	if err != nil {
		return user, err
	}
	return user, nil
}

func (u *Database) UpdateUser(login string, t string) error {
	_, err := u.Pg.Exec(context.Background(), `UPDATE rewusers SET refresh_token = $1 WHERE login = $2;`, t, login)
	if err != nil {
		return err
	}
	return nil
}
