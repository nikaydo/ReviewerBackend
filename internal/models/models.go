package models

import (
	"time"
)

type User struct {
	Uuid         string `json:"uuid,omitempty"`
	Login        string `json:"login"`
	Pass         string `json:"pass"`
	RefreshToken string `json:"refresh,omitempty"`
}

type UserTab struct {
	Uuid     string    `json:"uuid,omitempty"`
	User     string    `json:"user"`
	Request  string    `json:"request"`
	Answer   string    `json:"answer"`
	Think    string    `json:"think"`
	Model    string    `json:"model"`
	Favorite bool      `json:"favorite"`
	Title    Title     `json:"title"`
	Date     time.Time `json:"date"`
}

type UserSettings struct {
	Uuid      string  `json:"uuid,omitempty"`
	Request   string  `json:"request"`
	MainPromt *string `json:"mainpromt"`
	Model     string  `json:"model"`
}

type CustomPromt struct {
	Uuid     string `json:"uuid"`
	UuidUser string `json:"uuidUser"`
	Name     string `json:"name"`
	Promt    string `json:"promt"`
}

type Title struct {
	Uuid       string  `json:"uuid,omitempty"`
	Request    *string `json:"request"`
	Title      *string `json:"title"`
	UuidReview string  `json:"uuidreview"`
}

type ResponseFromApi struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type ResponseFromAI struct {
	Response string `json:"response"`
	Think    string `json:"think"`
}
