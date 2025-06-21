package textanalize

import (
	"encoding/json"
	"fmt"
	"io"
	"main/internal/config"
	"main/internal/models"
	"net/http"
	"net/url"
	"strings"
)

func Generate(reqs string, e config.Env) (models.Response, error) {
	apiKey := e.EnvMap["TEXTANALYZE"]
	endpoint := "https://api.textrazor.com/"

	data := url.Values{}
	data.Set("extractors", "entities,entailments")
	data.Set("text", reqs)

	client := &http.Client{}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return models.Response{}, err
	}

	req.Header.Set("x-textrazor-key", apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return models.Response{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return models.Response{}, err
	}

	var response models.Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
		fmt.Printf("Raw response: %s\n", string(body))
		return models.Response{}, err
	}

	if !response.Ok {
		fmt.Println("API request failed")
		return models.Response{}, err
	}

	return response, nil

}
