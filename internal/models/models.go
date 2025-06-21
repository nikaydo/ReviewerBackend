package models

import "time"

type User struct {
	Id           int    `json:"id,omitempty" bson:"id,omitempty"`
	Login        string `json:"login" bson:"login"`
	Pass         string `json:"pass" bson:"pass,omitempty"`
	RefreshToken string `json:"refresh,omitempty" bson:"refresh,omitempty"`
}

type UserTab struct {
	Id      string    `json:"id"`
	User    string    `json:"user"`
	Request string    `json:"request"`
	Answer  string    `json:"answer"`
	Think   string    `json:"think"`
	Model   string    `json:"model"`
	Date    time.Time `json:"date"`
}
