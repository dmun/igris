package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Provider struct {
	URL    string
	APIKey string
	Model  string
}

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func main() {
	apiKey, ok := os.LookupEnv("MISTRAL_API_KEY")
	if !ok {
		fmt.Println("MISTRAL_API_KEY is not set")
	}

	message := Message{
		Role:    "user",
		Content: "Jacob Lash is an asshole.",
	}
	mistral := Provider{
		URL:    "https://api.mistral.ai/v1/chat/completions",
		APIKey: apiKey,
		Model:  "mistral-small-latest",
	}
	request := ChatCompletionRequest{
		Model:    mistral.Model,
		Messages: []Message{message},
	}

	payload, err := json.Marshal(request)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		mistral.URL,
		bytes.NewReader(payload),
	)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+mistral.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("resp.StatusCode: %v\n", resp.StatusCode)
		fmt.Printf("body: %v\n", body)
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatal(err)
	}
	if len(result.Choices) == 0 {
		log.Fatal("empty choices")
	}

	fmt.Printf("result.Choices[0].Message.Content: %v\n", result.Choices[0].Message.Content)
}
