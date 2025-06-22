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
	Id      string    `json:"id,omitempty"`
	User    string    `json:"user"`
	Request string    `json:"request"`
	Answer  string    `json:"answer"`
	Think   string    `json:"think"`
	Model   string    `json:"model"`
	Date    time.Time `json:"date"`
}

type UserSettings struct {
	Id       string `json:"id"`
	Username string `json:"username"`
	Request  string `json:"request"`
	Model    string `json:"model"`
}

type Response struct {
	ResponseData ResponseData `json:"response"`
	Time         float64      `json:"time"`
	Ok           bool         `json:"ok"`
}

type ResponseData struct {
	Sentences          []Sentence   `json:"sentences"`
	Language           string       `json:"language"`
	LanguageIsReliable bool         `json:"languageIsReliable"`
	Entities           []Entity     `json:"entities"`
	Entailments        []Entailment `json:"entailments"`
}

type Sentence struct {
	Position int    `json:"position"`
	Words    []Word `json:"words"`
}

type Word struct {
	Position     int    `json:"position"`
	StartingPos  int    `json:"startingPos"`
	EndingPos    int    `json:"endingPos"`
	Stem         string `json:"stem"`
	Token        string `json:"token"`
	PartOfSpeech string `json:"partOfSpeech"`
}

type Entity struct {
	ID              int      `json:"id"`
	Type            []string `json:"type,omitempty"`
	MatchingTokens  []int    `json:"matchingTokens"`
	EntityID        string   `json:"entityId"`
	FreebaseTypes   []string `json:"freebaseTypes"`
	ConfidenceScore float64  `json:"confidenceScore"`
	WikiLink        string   `json:"wikiLink"`
	MatchedText     string   `json:"matchedText"`
	FreebaseID      string   `json:"freebaseId"`
	RelevanceScore  float64  `json:"relevanceScore"`
	EntityEnglishID string   `json:"entityEnglishId"`
	StartingPos     int      `json:"startingPos"`
	EndingPos       int      `json:"endingPos"`
	WikidataID      string   `json:"wikidataId"`
	WikidataTypes   []string `json:"wikidataTypes"`
}

type Entailment struct {
	ID            int     `json:"id"`
	WordPositions []int   `json:"wordPositions"`
	EntailedTree  string  `json:"entailedTree"`
	Score         float64 `json:"score"`
	ContextScore  float64 `json:"contextScore"`
	PriorScore    float64 `json:"priorScore"`
}
