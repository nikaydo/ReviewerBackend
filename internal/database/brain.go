package database

import (
	"context"
)

func (d *Database) Remember(uuid, text string) error {
	_, err := d.Pg.Exec(context.Background(), "INSERT INTO "+d.Env.EnvMap["DB_USER_BRAIN"]+" (uuid,memory) VALUES ($1,$2)", uuid, text)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) KeepInMind(text, uuid string) error {
	_, err := d.Pg.Exec(context.Background(), "Update "+d.Env.EnvMap["DB_USER_BRAIN"]+" SET memory = $1 WHERE uuid =  $2", uuid, text)
	if err != nil {
		return err
	}
	return nil
}

func (d *Database) Recall(uuid string) (string, error) {
	var memory string
	err := d.Pg.QueryRow(context.Background(), "SELECT memory from "+d.Env.EnvMap["DB_USER_BRAIN"]+" WHERE uuid = $1", uuid).Scan(&memory)
	if err != nil {
		return "", err
	}
	return memory, nil
}
