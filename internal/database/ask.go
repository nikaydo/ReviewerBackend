package database

import "context"

func (d *Database) ReviewTitleAdd(uuidReview, title, request string) error {
	_, err := d.Pg.Exec(context.Background(), `
		INSERT INTO `+d.Env.EnvMap["DB_REVIEW_ASK"]+` (uuidReview, title, request)
		VALUES ($1, $2, $3)
		ON CONFLICT (uuidReview) DO UPDATE
		SET title = EXCLUDED.title,
		    request = EXCLUDED.request
	`, uuidReview, title, request)
	return err
}

func (d *Database) ReviewTitleUpdate(title, uuid string) error {
	_, err := d.Pg.Exec(context.Background(), "UPDATE "+d.Env.EnvMap["DB_REVIEW_ASK"]+" SET title = $1 WHERE uuidReview = $2", title, uuid)
	if err != nil {
		return err
	}
	return nil
}
