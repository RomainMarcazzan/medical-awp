package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math" // Added for math.Sqrt
	"net/http"
	"os"            // Added for file system operations
	"path/filepath" // Added for path manipulation
	"sort"          // Added for sorting slices
	"strings"       // Added for strings.Fields

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

// OllamaEmbeddingRequest defines the structure for the Ollama API embedding request
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse defines the structure for the Ollama API embedding response
type OllamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// getOllamaEmbedding calls the Ollama API to get an embedding for the given text.
// It is not bound to the frontend and is intended for internal backend use.
func (a *App) getOllamaEmbedding(text string) ([]float64, error) {
	log.Printf("Requesting embedding for text (first 100 chars): %s...", text[:min(len(text), 100)])

	requestBody := OllamaEmbeddingRequest{
		Model:  embeddingModelName, // Uses the package-level constant
		Prompt: text,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshalling embedding request body: %v", err)
		return nil, fmt.Errorf("could not process embedding request (marshal): %w", err)
	}

	apiEndpoint := ollamaApiUrl + "/embeddings" // Uses the package-level constant
	log.Printf("Sending embedding request to Ollama endpoint: %s. Payload: %s", apiEndpoint, string(jsonBody))

	resp, err := http.Post(apiEndpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("Error making POST request to Ollama for embeddings: %v", err)
		return nil, fmt.Errorf("could not connect to Ollama service for embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Ollama embeddings API responded with status: %s - %s", resp.Status, string(responseBodyBytes))
		return nil, fmt.Errorf("Ollama embeddings API error (%s): %s", resp.Status, string(responseBodyBytes))
	}

	var ollamaEmbeddingResp OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaEmbeddingResp); err != nil {
		log.Printf("Error unmarshalling embedding response: %v", err)
		return nil, fmt.Errorf("could not parse Ollama embedding response: %w", err)
	}

	if len(ollamaEmbeddingResp.Embedding) == 0 {
		log.Println("Received empty embedding from Ollama.")
		return nil, fmt.Errorf("received empty embedding from Ollama")
	}

	log.Printf("Successfully received embedding of dimension: %d", len(ollamaEmbeddingResp.Embedding))
	return ollamaEmbeddingResp.Embedding, nil
}

// min returns the smaller of x or y.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// chunkText splits a given text into smaller chunks based on word count.
// chunkSize is the number of words per chunk.
// overlap is the number of words to overlap between consecutive chunks.
func chunkText(text string, chunkSize int, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 200 // Default chunk size in words
	}
	if overlap < 0 {
		overlap = 0 // Default overlap in words
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4 // Ensure overlap is less than chunk size, e.g., 25% if invalid
	}

	words := strings.Fields(text)
	var chunks []string

	if len(words) == 0 {
		return chunks
	}

	for i := 0; i < len(words); {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))

		i += chunkSize - overlap
		if i >= len(words) && end < len(words) { // Ensure the very last part is captured if loop step overshoots
		}
		// If i steps into the last chunk territory but doesn't cover it fully, the next iteration's `end = len(words)` will handle it.
		// The loop condition `i < len(words)` ensures we don't go out of bounds for `words[i:end]` start.
	}
	return chunks
}

// LoadPersonalData is a Wails-bindable method that processes .txt files from a directory,
// chunks them, generates embeddings, and stores them in memory.
func (a *App) LoadPersonalData(directoryPath string) string {
	log.Printf("Starting to load personal data from directory: %s", directoryPath)
	a.documentStore = make([]DocumentChunk, 0) // Clear existing store
	a.nextDocumentID = 1                       // Reset ID counter

	filesProcessed := 0
	totalChunksLoaded := 0

	// Recommended way to read directory contents in Go
	dirEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		log.Printf("Error reading directory %s: %v", directoryPath, err)
		return fmt.Sprintf("Error reading directory: %v", err)
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".txt" {
			continue // Skip directories and non-.txt files
		}

		fileName := entry.Name()
		filePath := filepath.Join(directoryPath, fileName)
		log.Printf("Processing file: %s", filePath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Error reading file %s: %v", filePath, err)
			continue // Skip this file
		}

		// Define chunking parameters (could be made configurable later)
		chunkSize := 200
		overlap := 50
		textChunks := chunkText(string(content), chunkSize, overlap)

		for i, chunkText := range textChunks {
			log.Printf("Processing chunk %d/%d for file %s (length: %d)", i+1, len(textChunks), fileName, len(chunkText))
			embedding, err := a.getOllamaEmbedding(chunkText)
			if err != nil {
				log.Printf("Error getting embedding for chunk from %s: %v. Skipping chunk.", fileName, err)
				continue // Skip this chunk
			}

			docChunk := DocumentChunk{
				ID:         a.nextDocumentID,
				Text:       chunkText,
				Embedding:  embedding,
				SourceFile: fileName,
			}
			a.documentStore = append(a.documentStore, docChunk)
			a.nextDocumentID++
			totalChunksLoaded++
		}
		filesProcessed++
	}

	statusMessage := fmt.Sprintf("Successfully processed %d files, loaded %d chunks into document store.", filesProcessed, totalChunksLoaded)
	log.Println(statusMessage)
	return statusMessage
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(vecA, vecB []float64) (float64, error) {
	if len(vecA) != len(vecB) {
		return 0, fmt.Errorf("vectors must have the same length (A: %d, B: %d)", len(vecA), len(vecB))
	}
	if len(vecA) == 0 {
		return 0, fmt.Errorf("vectors must not be empty")
	}

	dotProduct := 0.0
	magnitudeA := 0.0
	magnitudeB := 0.0

	for i := 0; i < len(vecA); i++ {
		dotProduct += vecA[i] * vecB[i]
		magnitudeA += vecA[i] * vecA[i]
		magnitudeB += vecB[i] * vecB[i]
	}

	magnitudeA = math.Sqrt(magnitudeA)
	magnitudeB = math.Sqrt(magnitudeB)

	if magnitudeA == 0 || magnitudeB == 0 {
		// If one vector is a zero vector, similarity is undefined or can be considered 0.
		// Depending on the use case, returning an error might also be appropriate.
		// For RAG, if a document chunk somehow had a zero embedding (unlikely with good models),
		// it would simply have zero similarity to any query.
		return 0, nil
	}

	return dotProduct / (magnitudeA * magnitudeB), nil
}

// rankedChunk holds a DocumentChunk and its similarity score for sorting.
type rankedChunk struct {
	chunk DocumentChunk
	score float64
}

// findRelevantChunks finds the top N most similar document chunks to a query embedding.
func (a *App) findRelevantChunks(queryEmbedding []float64, topN int) []DocumentChunk {
	if len(a.documentStore) == 0 {
		log.Println("Document store is empty. Cannot find relevant chunks.")
		return []DocumentChunk{}
	}
	if topN <= 0 {
		topN = 3 // Default to top 3 if not specified or invalid
	}

	var rankedChunks []rankedChunk

	for _, chunk := range a.documentStore {
		if len(chunk.Embedding) == 0 {
			log.Printf("Skipping chunk ID %d from %s due to empty embedding.", chunk.ID, chunk.SourceFile)
			continue
		}
		similarity, err := cosineSimilarity(queryEmbedding, chunk.Embedding)
		if err != nil {
			log.Printf("Error calculating similarity for chunk ID %d (%s): %v. Skipping.", chunk.ID, chunk.SourceFile, err)
			continue
		}
		rankedChunks = append(rankedChunks, rankedChunk{chunk: chunk, score: similarity})
	}

	// Sort chunks by score in descending order
	sort.Slice(rankedChunks, func(i, j int) bool {
		return rankedChunks[i].score > rankedChunks[j].score
	})

	numToReturn := min(topN, len(rankedChunks))
	resultChunks := make([]DocumentChunk, numToReturn)
	for i := 0; i < numToReturn; i++ {
		resultChunks[i] = rankedChunks[i].chunk
		log.Printf("Relevant chunk %d: ID %d, Source: %s, Score: %.4f", i+1, rankedChunks[i].chunk.ID, rankedChunks[i].chunk.SourceFile, rankedChunks[i].score)
	}

	return resultChunks
}

// sendErrorEvent is a helper to emit an error event to the frontend.
func (a *App) sendErrorEvent(errMessage string) {
	log.Println("Sending error event to frontend:", errMessage)
	runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{Error: errMessage, Done: true})
}

// askOllamaChatRaw sends a request to Ollama /api/chat and handles streaming response via events.
// This function will be called by HandleMessage for both RAG and non-RAG flows.
func (a *App) askOllamaChatRaw(messages []OllamaChatMessage) {
	requestBody := OllamaChatRequest{
		Model:    chatModelName,
		Messages: messages,
		Stream:   true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		errMsg := fmt.Sprintf("Error marshalling request body: %v", err)
		a.sendErrorEvent(errMsg)
		return
	}

	log.Printf("Sending request to Ollama: %s", string(jsonBody))

	resp, err := http.Post(ollamaApiUrl+"/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		errMsg := fmt.Sprintf("Error making POST request to Ollama: %v", err)
		a.sendErrorEvent(errMsg)
		return
	}

	if resp.StatusCode != http.StatusOK {
		responseBodyBytes, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		errMsg := fmt.Sprintf("Ollama API responded with status: %s - %s", resp.Status, string(responseBodyBytes))
		a.sendErrorEvent(errMsg)
		return
	}

	// Process the stream in a goroutine
	go func() {
		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					log.Println("Stream finished (EOF).")
					runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{Done: true})
					break
				}
				errMsg := fmt.Sprintf("Error reading stream: %v", err)
				a.sendErrorEvent(errMsg)
				break
			}

			var ollamaChunk OllamaChatResponse
			if err := json.Unmarshal(line, &ollamaChunk); err != nil {
				log.Printf("Error unmarshalling stream chunk '%s': %v. Skipping.", string(line), err)
				// Optionally send an error event or try to continue. For now, we log and continue.
				// a.sendErrorEvent(fmt.Sprintf("Error parsing chunk: %v", err))
				continue
			}

			// log.Printf("Sending chunk to frontend: '%s' (Done: %v)", ollamaChunk.Message.Content, ollamaChunk.Done)
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
}

func (a *App) HandleMessage(userInput string) string {
	log.Printf("Received message from user: %s", userInput)

	var messagesToOllama []OllamaChatMessage

	if len(a.documentStore) == 0 {
		log.Println("Document store is empty. Proceeding with non-RAG chat.")
		messagesToOllama = []OllamaChatMessage{{
			Role:    "user",
			Content: userInput,
		}}
	} else {
		log.Println("Document store found. Proceeding with RAG chat.")
		queryEmbedding, err := a.getOllamaEmbedding(userInput)
		if err != nil {
			errMsg := fmt.Sprintf("Error getting embedding for user query: %v", err)
			a.sendErrorEvent(errMsg)
			return "" // Return empty as per existing pattern, error sent via event
		}

		topN := 3 // Configurable: number of relevant chunks to retrieve
		relevantChunks := a.findRelevantChunks(queryEmbedding, topN)

		if len(relevantChunks) == 0 {
			log.Println("No relevant chunks found for the query. Falling back to non-RAG chat.")
			messagesToOllama = []OllamaChatMessage{{
				Role:    "user",
				Content: userInput,
			}}
		} else {
			log.Printf("Found %d relevant chunks for the query.", len(relevantChunks))
			var promptBuilder strings.Builder
			promptBuilder.WriteString("Based on the following information from your documents:\n\n")
			for _, chunk := range relevantChunks {
				promptBuilder.WriteString(fmt.Sprintf("--- Context from %s ---\n%s\n\n", chunk.SourceFile, chunk.Text))
			}
			promptBuilder.WriteString("--- End of Context ---\n\nPlease answer this question: " + userInput)
			augmentedPrompt := promptBuilder.String()
			log.Printf("Augmented prompt:\n%s", augmentedPrompt)

			messagesToOllama = []OllamaChatMessage{{
				Role:    "user",
				Content: augmentedPrompt,
			}}
		}
	}

	// Call the refactored function to handle the Ollama call and streaming
	a.askOllamaChatRaw(messagesToOllama)

	return "" // Indicate success, streaming handled by events/goroutine
}
