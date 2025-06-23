package models

import (
	"time"
)

type User struct {
	Id           int    `json:"id,omitempty" `
	Login        string `json:"login"`
	Pass         string `json:"pass" `
	RefreshToken string `json:"refresh,omitempty"`
}

type UserTab struct {
	Id       string    `json:"id,omitempty"`
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
	Id       string `json:"id,omitempty"`
	Username string `json:"username"`
	Request  string `json:"request"`
	Model    string `json:"model"`
}

type Title struct {
	Id       string  `json:"id,omitempty"`
	Request  *string `json:"request"`
	Title    *string `json:"title"`
	IdReview string  `json:"idreview"`
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
