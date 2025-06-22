package database

import (
	"context"
	"main/internal/models"
)

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

// Settings
func (d *Database) GetSettings(username string) (models.UserSettings, error) {
	rows := d.Pg.QueryRow(context.Background(), "SELECT * from userSetting where username = $1", username)
	var u models.UserSettings
	if err := rows.Scan(&u.Id, &u.Username, &u.Request, &u.Model); err != nil {
		return u, err
	}
	return u, nil
}
func (d *Database) SaveSettings(username, request, model string) error {
	_, err := d.Pg.Exec(context.Background(), "INSERT INTO userSetting (username, request, model) VALUES ($1, $2, $3)", username, request, model)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) UpdateSettings(username, request, model string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE userSetting SET request = $1, model = $2 WHERE username = $3", request, model, username)
	if err != nil {
		return err
	}
	return nil
}
