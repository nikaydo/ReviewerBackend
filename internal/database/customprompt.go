package database

import (
	"context"
	"main/internal/models"
)

func (d *Database) CustomPromtAdd(uuidUser, name, promt string) error {
	_, err := d.Pg.Exec(context.Background(), "INSERT INTO "+d.Env.EnvMap["DB_USER_PROMT"]+" (uuidUser,name,promt) VALUES ($1,$2,$3)", uuidUser, name, promt)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) CustomPromtUpdate(uuid, name, promt string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE "+d.Env.EnvMap["DB_USER_PROMT"]+" SET (name,promt) = ($1,$2) WHERE uuidUniq = $3", name, promt, uuid)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) CustomPromtGet(uuidUser string) ([]models.CustomPromt, error) {
	rows, err := d.Pg.Query(context.Background(), "SELECT * FROM "+d.Env.EnvMap["DB_USER_PROMT"]+" WHERE uuidUser = $1", uuidUser)
	if err != nil {
		return nil, err
	}
	var ls []models.CustomPromt
	for rows.Next() {
		var c models.CustomPromt
		rows.Scan(&c.UuidUser, &c.Uuid, &c.Name, &c.Promt)
		ls = append(ls, c)
	}
	return ls, nil
}

func (d *Database) CustomPromtDel(uuid string) error {
	_, err := d.Pg.Exec(context.Background(), "DELETE FROM "+d.Env.EnvMap["DB_USER_PROMT"]+" WHERE uuidUniq = $1", uuid)
	if err != nil {
		return err
	}
	return nil
}
