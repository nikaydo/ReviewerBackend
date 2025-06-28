package database

import (
	"context"
	"database/sql"
	"main/internal/models"
	"time"
)

func (d *Database) ReviewAdd(uuid, request, answer, think, model string) error {
	_, err := d.Pg.Exec(context.Background(), `
		INSERT INTO 
		`+d.Env.EnvMap["DB_REVIEW"]+` 
		(uuid, request,answer,think,date,model,favorite) 
		VALUES 
		($1, $2, $3, $4, $5, $6, $7)
		`, uuid, request, answer, think, time.Now(), model, false)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) ReviewFavorite(uuid, favorite, uuidUniq string) error {
	f := false
	if favorite == "true" {
		f = true
	}
	_, err := d.Pg.Exec(context.Background(), "UPDATE "+d.Env.EnvMap["DB_REVIEW"]+" SET favorite = $1 WHERE uuid = $2 AND uuidUniq = $3", f, uuid, uuidUniq)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) ReviewSum(uuid, uuidUniq string) (string, string, error) {
	var mainPromt string
	var customPromt string
	err := d.Pg.QueryRow(context.Background(), `SELECT mainPromt FROM `+d.Env.EnvMap["DB_USER_SETTING"]+` WHERE uuid = $1`, uuid).Scan(&mainPromt)
	if err != nil {
		return "", "", err

	}
	err = d.Pg.QueryRow(context.Background(), `SELECT promt FROM `+d.Env.EnvMap["DB_USER_PROMT"]+` WHERE uuidUniq = $1`, uuidUniq).Scan(&customPromt)
	if err != sql.ErrNoRows {
		err = nil
		return mainPromt, customPromt, nil
	}
	if err != nil {
		return "", "", err
	}
	return mainPromt, customPromt, nil
}

func (d *Database) ReviewGet(uuid string) ([]models.UserTab, error) {
	ctx := context.Background()

	tx, err := d.Pg.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT
			rt.uuidUniq,
			rt.uuid,
			rt.request,
			rt.answer,
			rt.think,
			rt.date,
			rt.model,
			rt.favorite,
			rtitle.title,
			rtitle.request AS title_request
		FROM `+d.Env.EnvMap["DB_REVIEW"]+`  rt
		LEFT JOIN `+d.Env.EnvMap["DB_REVIEW_ASK"]+` rtitle ON rtitle.uuidReview = rt.uuid
		WHERE rt.uuid = $1;
	`, uuid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ls []models.UserTab
	for rows.Next() {
		var u models.UserTab
		if err := rows.Scan(
			&u.Uuid,
			&u.User,
			&u.Request,
			&u.Answer,
			&u.Think,
			&u.Date,
			&u.Model,
			&u.Favorite,
			&u.Title.Title,
			&u.Title.Request,
		); err != nil {
			return ls, err
		}
		ls = append(ls, u)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ls, nil
}

func (d *Database) UpdateReview(user_uuid, r, uuid string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE "+d.Env.EnvMap["DB_REVIEW"]+" SET answer = $1 WHERE uuid = $2 AND uuid = $3", r, user_uuid, uuid)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) ReviewGetOne(user_uuid, uuid string) (models.UserTab, error) {
	rows := d.Pg.QueryRow(context.Background(), "SELECT uuid,request,answer,think,date,model,favorite FROM "+d.Env.EnvMap["DB_REVIEW"]+" WHERE uuid = $1 AND uuid = $2", user_uuid, uuid)
	var u models.UserTab
	if err := rows.Scan(&u.Uuid, &u.Request, &u.Answer, &u.Think, &u.Date, &u.Model, &u.Favorite); err != nil {
		return u, err
	}
	return u, nil
}

func (d *Database) ReviewDelete(user_uuid, uuid string) error {
	_, err := d.Pg.Exec(context.Background(), "DELETE FROM "+d.Env.EnvMap["DB_REVIEW"]+" WHERE uuid = $1 AND uuid = $2", user_uuid, uuid)
	if err != nil {
		return err
	}
	return nil
}
