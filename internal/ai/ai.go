package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"main/internal/models"
	"net/http"
	"strings"
)

type ResponseFromAI struct {
	Response string `json:"response"`
	Think    string `json:"think"`
}

func Generate(model, api, reqv, promt string, sys bool) (ResponseFromAI, error) {
	var R ResponseFromAI
	url := "https://api.mistral.ai/v1/chat/completions"

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": reqv,
			},
			{
				"role":    "system",
				"content": promt,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return R, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return R, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+api)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return R, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response models.ResponseFromApi
	if err := json.Unmarshal(body, &response); err != nil {
		return R, err
	}

	if model == "magistral-medium-2506" {
		R, err := GenWithThink(response)
		if err != nil {
			return R, err
		}
		return R, nil
	}

	R.Response = response.Choices[0].Message.Content
	return R, nil
}

func GenWithThink(response models.ResponseFromApi) (ResponseFromAI, error) {
	var R ResponseFromAI
	if len(response.Choices) == 0 {
		return R, fmt.Errorf("empty response")
	}
	otvet := strings.Split(response.Choices[0].Message.Content, "</think>")
	answer := strings.Split(otvet[1], "\\boxed{")
	R.Response = strings.TrimSpace(answer[0])
	f := strings.ReplaceAll(R.Response, "—", "-")
	R.Response = f
	R.Think = strings.TrimSpace(otvet[0])
	return R, nil
}
