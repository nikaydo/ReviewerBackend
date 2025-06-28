package handles

import (
	"encoding/json"
	"fmt"
	"log"
	"main/internal/config"
	"main/internal/jwt"
	"net/http"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func writeJSONResponse(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_, err = w.Write(jsonData)
	if err != nil {
		log.Println("error writing response", err)
	}
}

func writeErrorResponse(w http.ResponseWriter, data error, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(data.Error()))
	if err != nil {
		log.Println("error writing response", err)
	}
}

func GetUsername(w http.ResponseWriter, r *http.Request, e config.Env) (string, string, error) {
	c, err := r.Cookie("jwt")
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("Unauthorized"), http.StatusUnauthorized)
		return "", "", err
	}
	uuid, username, err := jwt.ValidateToken(c.Value, e.EnvMap["SECRET"])
	if err != nil {
		writeErrorResponse(w, fmt.Errorf("Unauthorized"), http.StatusUnauthorized)
		return "", "", err
	}
	return uuid, username, nil
}

func MakeCookie(name, value string, t time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		//Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		Expires:  time.Now().Add(t),
		MaxAge:   int(t.Seconds()),
	}
}

func FindChange(originalText, changedText string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(originalText, changedText, false)
	var additions string
	for _, diff := range diffs {
		if diff.Type == diffmatchpatch.DiffInsert {
			additions += "| " + diff.Text
		}
	}
	return additions
}

func makePrompt(main, mem, preset string) string {
	glueForPreset := " важно что бы твой ответ соответствовал этим критериям: " + preset
	glueForMem := " используй эту инвормацию для написания как информацию что бы сделать отзыв более подходящим под критерии " + mem
	return main + glueForMem + glueForPreset
}
