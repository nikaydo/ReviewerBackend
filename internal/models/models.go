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
	Date     time.Time `json:"date"`
}

type UserSettings struct {
	Id       string `json:"id"`
	Username string `json:"username"`
	Request  string `json:"request"`
	Model    string `json:"model"`
}

type ResponseFromApi struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
