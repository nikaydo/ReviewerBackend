package handles

import (
	"main/internal/database"
	"main/internal/models"
)

type Handlers struct {
	Pg    database.Database
	Queue *models.List
}
