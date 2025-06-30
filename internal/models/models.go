package models

import (
	"sync"
	"time"

	"github.com/google/uuid"
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
	Uuid       string  `json:"uuid,omitempty"`
	Request    string  `json:"request"`
	InProgress string  `json:"inprogress"`
	MainPromt  *string `json:"mainpromt"`
	Model      string  `json:"model"`
	Count      int     `json:"count"`
	Memory     *bool   `json:"memory"`
}

type Memory struct {
	Mem string `json:"memory"`
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

type List struct {
	sync.Mutex
	Request []Enquiry
}

/*
структура запроса в очереди

	QueryUuid - это индитификатор запроса в очереди
	Uuid - пользователь чей это запрос.
	Model - модель которая исползуеться в запросе.
	Request - сам запрос который отпарвил пользователь.
	System - системный промт.
	Assistant - предыдущие ответы ии на запросы пользователя.
	IsSystem - булевое значение включать ли системный промт в запрос.
	IsAssistant - булевое значение включать ли предыдущие ответы в запрос.
	Type  - куда сохранить ответ от ии
		1 - в отзывы
		2 - в ответ на вопрос
*/
type Enquiry struct {
	QueryUuid   uuid.UUID
	Uuid        string
	AskUuid     string
	Model       string
	Request     string
	System      string
	Assistant   string
	Memory      bool
	IsSystem    bool
	IsAssistant bool
	Type        int
}

type MeInEnquiry struct {
	Position int `json:"Position"`
	Total    int `json:"Total"`
}

type ReturnUuid struct {
	UuidQuery uuid.UUID `json:"uuid"`
}
