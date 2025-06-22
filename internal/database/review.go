package database

import (
	"context"
	"main/internal/models"
	"time"
)

func (d *Database) ReviewAdd(user, request, answer, think, model string) error {
	_, err := d.Pg.Exec(context.Background(), `
	INSERT INTO 
		reviewTears 
		(username, request,answer,think,date,model,favorite) 
		VALUES 
		($1, $2, $3, $4, $5, $6, $7)
		`, user, request, answer, think, time.Now(), model, false)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) ReviewFavorite(username, favorite, id string) error {
	f := false
	if favorite == "true" {
		f = true
	}
	_, err := d.Pg.Exec(context.Background(), "UPDATE reviewTears SET favorite = $1 WHERE username = $2 AND id = $3", f, username, id)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) ReviewGet(user string) ([]models.UserTab, error) {
	rows, err := d.Pg.Query(context.Background(), "SELECT id,request,answer,think,date,model,favorite FROM reviewTears WHERE username = $1", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ls []models.UserTab
	for rows.Next() {
		var u models.UserTab
		if err := rows.Scan(&u.Id, &u.Request, &u.Answer, &u.Think, &u.Date, &u.Model, &u.Favorite); err != nil {
			return ls, err
		}
		ls = append(ls, u)
	}
	return ls, nil
}

func (d *Database) UpdateReview(username, r, id string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE reviewTears SET answer = $1 WHERE username = $2 AND id = $3", r, username, id)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) ReviewGetOne(user, id string) (models.UserTab, error) {
	rows := d.Pg.QueryRow(context.Background(), "SELECT id,request,answer,think,date,model,favorite FROM reviewTears WHERE username = $1 AND id = $2", user, id)
	var u models.UserTab
	if err := rows.Scan(&u.Id, &u.Request, &u.Answer, &u.Think, &u.Date, &u.Model, &u.Favorite); err != nil {
		return u, err
	}
	return u, nil
}

func (d *Database) ReviewDelete(user, id string) error {
	_, err := d.Pg.Exec(context.Background(), "DELETE FROM reviewTears WHERE username = $1 AND id = $2", user, id)
	if err != nil {
		return err
	}
	return nil
}
