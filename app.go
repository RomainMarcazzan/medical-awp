package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	ollamaApiUrl       = "http://localhost:11434/api" // Base URL for Ollama API
	chatModelName      = "llama3"                     // Model for chat completions
	embeddingModelName = "nomic-embed-text"           // Model for generating embeddings
)

// DocumentChunk defines the structure for a piece of text from a document.
type DocumentChunk struct {
	ID         int       `json:"id"`
	Text       string    `json:"text"`
	Embedding  []float64 `json:"embedding"`   // Stores the vector embedding of the text
	SourceFile string    `json:"source_file"` // Original file this chunk came from
}

// App struct
type App struct {
	ctx            context.Context
	documentStore  []DocumentChunk
	nextDocumentID int
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		documentStore:  make([]DocumentChunk, 0),
		nextDocumentID: 1,
	}
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

// OllamaChatResponse defines the structure for each chunk in the Ollama API stream
type OllamaChatResponse struct {
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   OllamaChatMessage `json:"message"`
	Done      bool              `json:"done"`
	// Other fields like total_duration, etc., appear in the final 'done' chunk
}

// OllamaStreamEvent is the payload sent to the frontend for each stream event
type OllamaStreamEvent struct {
	Content string `json:"content,omitempty"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

func (a *App) HandleMessage(userInput string) string {
	log.Printf("Received message from user: %s", userInput)

	// Use package-level constants
	// ollamaApiUrl is now a const: ollamaApiUrl
	// chatModelName is now a const: chatModelName

	requestBody := OllamaChatRequest{
		Model: chatModelName, // Use const
		Messages: []OllamaChatMessage{
			{
				Role:    "user",
				Content: userInput,
			},
		},
		Stream: true, // Enable streaming
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshalling request body: %v", err)
		return "Error: Could not process your request (marshal)."
	}

	log.Printf("Sending request to Ollama: %s", string(jsonBody))

	resp, err := http.Post(ollamaApiUrl+"/chat", "application/json", bytes.NewBuffer(jsonBody)) // Adjusted to use base const
	if err != nil {
		log.Printf("Error making POST request to Ollama: %v", err)
		return "Error: Could not connect to Ollama service."
	}
	// Defer closing body in the goroutine after it's done with it.

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // Close body here as we are not streaming it
		log.Printf("Ollama API responded with status: %s - %s", resp.Status, string(responseBodyBytes))
		return fmt.Sprintf("Error: Ollama service responded with an error (%s).", resp.Status)
	}

	// Process the stream in a goroutine
	go func() {
		defer resp.Body.Close() // Ensure body is closed when goroutine finishes
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n') // Corrected: use '\n' instead of '\\n'
			if err != nil {
				if err == io.EOF {
					log.Println("Stream finished (EOF).")
					// Ensure a final 'done' event is sent if not already by Ollama
					runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{Done: true})
					break
				}
				log.Printf("Error reading stream: %v", err)
				runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{Error: "Error reading stream: " + err.Error(), Done: true})
				break
			}

			var ollamaChunk OllamaChatResponse
			if err := json.Unmarshal(line, &ollamaChunk); err != nil {
				log.Printf("Error unmarshalling stream chunk '%s': %v", string(line), err)
				// Optionally send an error event or try to continue
				// runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{Error: "Error parsing chunk.", Done: true})
				continue
			}

			log.Printf("Sending chunk to frontend: '%s' (Done: %v)", ollamaChunk.Message.Content, ollamaChunk.Done)
			runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{
				Content: ollamaChunk.Message.Content,
				Done:    ollamaChunk.Done,
			})

			if ollamaChunk.Done {
				log.Println("Stream officially done according to Ollama chunk.")
				break
			}
		}
	}()

	return "" // Indicate success, streaming handled by events
}
