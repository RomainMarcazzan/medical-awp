package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// OllamaChatMessage defines the structure for a message in the Ollama API
type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatRequest defines the structure for the Ollama API chat request
type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

// OllamaChatResponse defines the structure for the Ollama API chat response
type OllamaChatResponse struct {
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   OllamaChatMessage `json:"message"`
	Done      bool              `json:"done"`
}

func (a *App) HandleMessage(userInput string) string {
	log.Printf("Received message from user: %s", userInput)

	ollamaApiUrl := "http://localhost:11434/api/chat"
	chatModelName := "llama3" // As defined in progress.md

	requestBody := OllamaChatRequest{
		Model: chatModelName,
		Messages: []OllamaChatMessage{
			{
				Role:    "user",
				Content: userInput,
			},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshalling request body: %v", err)
		return "Error: Could not process your request."
	}

	log.Printf("Sending request to Ollama: %s", string(jsonBody))

	resp, err := http.Post(ollamaApiUrl, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("Error making POST request to Ollama: %v", err)
		return "Error: Could not connect to Ollama service."
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading Ollama response body: %v", err)
		return "Error: Could not read response from Ollama."
	}

	log.Printf("Received response from Ollama: Status %s, Body: %s", resp.Status, string(responseBody))

	if resp.StatusCode != http.StatusOK {
		log.Printf("Ollama API responded with status: %s - %s", resp.Status, string(responseBody))
		return fmt.Sprintf("Error: Ollama service responded with an error (%s).", resp.Status)
	}

	var ollamaResponse OllamaChatResponse
	err = json.Unmarshal(responseBody, &ollamaResponse)
	if err != nil {
		log.Printf("Error unmarshalling Ollama response: %v", err)
		log.Printf("Ollama raw response: %s", string(responseBody))
		return "Error: Could not parse response from Ollama."
	}

	if ollamaResponse.Message.Content == "" {
		log.Printf("Ollama response message content is empty. Full response: %+v", ollamaResponse)
		return "Error: Received an empty message from Ollama."
	}

	log.Printf("AI Response: %s", ollamaResponse.Message.Content)
	return ollamaResponse.Message.Content
}
