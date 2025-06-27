package database

import (
	"context"
	"database/sql"
	"errors"
	"main/internal/models"
	"strings"
)

func (u *Database) CreateUser(Login, Pass string) (string, error) {
	var uuid string
	err := u.Pg.QueryRow(context.Background(), `
		INSERT INTO `+u.Env.EnvMap["DB_USER"]+`  
		(login,password,refresh_token)
		VALUES ($1,$2,$3)
		RETURNING uuid;`, Login, Pass, "").Scan(&uuid)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value") {
			return uuid, errors.New("login alredy exist")
		}
		return uuid, err
	}
	_, err = u.Pg.Exec(context.Background(), "INSERT INTO "+u.Env.EnvMap["DB_USER_SETTING"]+" (uuid,mainPromt, request, model) VALUES ($1, $2, $3,$4)", uuid, u.Env.EnvMap["MAIN_PROMT_DEFAULT"], "", "")
	if err != nil {
		return uuid, err
	}
	return uuid, nil
}

func (u *Database) CheckUser(Login, Pass string, pass bool) (models.User, error) {
	var err error
	var user models.User
	if pass {
		err = u.Pg.QueryRow(context.Background(), `SELECT * FROM `+u.Env.EnvMap["DB_USER"]+` WHERE login = $1 AND password = $2;`, Login, Pass).Scan(&user.Uuid, &user.Login, &user.Pass, &user.RefreshToken)
	} else {
		err = u.Pg.QueryRow(context.Background(), `SELECT * FROM `+u.Env.EnvMap["DB_USER"]+` WHERE login = $1;`, Login).Scan(&user.Uuid, &user.Login, &user.Pass, &user.RefreshToken)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return user, errors.New("invalid login or password")
		}
		return user, err
	}
	return user, nil
}

func (u *Database) UpdateUser(login string, t string) error {
	_, err := u.Pg.Exec(context.Background(), `UPDATE `+u.Env.EnvMap["DB_USER"]+` SET refresh_token = $1 WHERE login = $2;`, t, login)
	if err != nil {
		return err
	}
	return nil
}
