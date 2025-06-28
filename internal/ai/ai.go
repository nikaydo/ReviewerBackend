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

func appendMsg(msg []map[string]string, role, content string) []map[string]string {
	return append(msg, []map[string]string{{
		"role":    role,
		"content": content,
	}}...)
}

func makeResponse(model, reqv, sysPromt, assistPromt string, sys, assist bool) ([]byte, error) {
	var msg []map[string]string
	if sys {
		msg = appendMsg(msg, "system", sysPromt)
	}
	if assist {
		msg = appendMsg(msg, "assistant", assistPromt)
	}
	msg = appendMsg(msg, "user", reqv)
	payload := map[string]any{
		"model":    model,
		"messages": msg,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return []byte{}, err
	}
	return jsonData, nil
}

func Generate(model, api, reqv, sysPromt, assistPromt string, sys, assist bool) (ResponseFromAI, error) {
	var R ResponseFromAI
	url := "https://api.mistral.ai/v1/chat/completions"
	jsonData, err := makeResponse(model, reqv, sysPromt, assistPromt, sys, assist)
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
		R, err := genWithThink(response)
		if err != nil {
			return R, err
		}
		return R, nil
	}
	R.Response = response.Choices[0].Message.Content
	return R, nil
}

func genWithThink(response models.ResponseFromApi) (ResponseFromAI, error) {
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
