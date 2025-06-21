package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func Generate(model, api, reqv string) (string, string) {
	type Response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	url := "https://api.mistral.ai/v1/chat/completions"
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": reqv,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Ошибка сериализации JSON:", err)
		return "", ""
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Ошибка создания запроса:", err)
		return "", ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+api)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Ошибка отправки запроса:", err)
		return "", ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Println("Ошибка разбора JSON:", err)
		fmt.Println(string(body))
		return "", ""
	}

	if len(response.Choices) == 0 {
		fmt.Println("Пустой ответ")
		return "", ""
	}
	otvet := strings.Split(response.Choices[0].Message.Content, "</think>")

	answer := strings.Split(otvet[1], "\\boxed{")
	return strings.TrimSpace(answer[0]), strings.TrimSpace(otvet[0])
}
