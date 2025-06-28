package database

import (
	"context"
	"main/internal/models"
)

/*
Получение настроек пользователя по нику. Возвращяет структуру UserSettings со значениями uuid, mainPromt, Request, Model
*/
func (d *Database) GetSettings(uuid string) (models.UserSettings, error) {
	rows := d.Pg.QueryRow(context.Background(), "SELECT * from "+d.Env.EnvMap["DB_USER_SETTING"]+" where uuid = $1", uuid)
	var u models.UserSettings
	if err := rows.Scan(&u.Uuid, &u.Request, &u.MainPromt, &u.Model); err != nil {
		return u, err
	}
	return u, nil
}

/*
Добавление настроек пользователя в базу данных. Добавляет uuid, request, model
*/
func (d *Database) SaveSettings(uuid, request, model string) error {
	_, err := d.Pg.Exec(context.Background(), "INSERT INTO "+d.Env.EnvMap["DB_USER_SETTING"]+" (uuid, request, model) VALUES ($1, $2, $3)", uuid, request, model)
	if err != nil {
		return err
	}
	return nil
}

/*
Обнолвние настроек пользователя. Проверка по uuid пользователя и добавление request, model
*/
func (d *Database) UpdateSettings(uuid, request, model string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE "+d.Env.EnvMap["DB_USER_SETTING"]+" SET request = $1, model = $2 WHERE uuid = $3", request, model, uuid)
	if err != nil {
		return err
	}
	return nil
}

/*
Обновление основного промта. Принимает сам промт mainPromt и uuid пользователя
*/
func (d *Database) ReviewTitleUpdatePromt(mainPromt, uuid string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE "+d.Env.EnvMap["DB_USER_SETTING"]+" SET mainPromt = $1 WHERE uuid = $2;", mainPromt, uuid)
	if err != nil {
		return err
	}
	return nil
}
