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
	"os"
	"path/filepath"
	"sort" // Added for sort.Slice
	"strings"
	"sync"         // Added for mutex
	"time"         // Added for timing
	"unicode/utf8" // Added for rune counting

	"github.com/go-ole/go-ole"
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
	Score      float64   // Added Score field for ranking
}

// SourceInfo defines the structure for information about a retrieved document chunk.
type SourceInfo struct {
	FileName string  `json:"fileName"`
	ChunkID  int     `json:"chunkId"`
	Score    float64 `json:"score"`
}

// App struct
type App struct {
	ctx            context.Context
	documentStore  []DocumentChunk
	nextDocumentID int
	mu             sync.Mutex // Mutex to protect documentStore and nextDocumentID
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		documentStore:  make([]DocumentChunk, 0), // Initialize documentStore
		nextDocumentID: 1,                        // Initialize nextDocumentID
		// mu will be zero-valued, which is ready for use
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Initialize COM for this goroutine (which Wails ensures is the main OS thread for startup)
	// COINIT_APARTMENTTHREADED is generally required for UI elements like dialogs.
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		log.Printf("FATAL: Failed to initialize COM: %v. Dialogs will not work.", err)
		// Depending on how critical dialogs are, you might panic or os.Exit here.
	} else {
		log.Println("COM initialized successfully for the main application thread.")
	}
}

// shutdown is called when the app is shutting down.
//
//lint:ignore U1000 Wails lifecycle method - shutdown is called when the app is shutting down.
func (a *App) shutdown(_ context.Context) { // Changed ctx to _
	log.Println("App shutting down. Uninitializing COM.")
	ole.CoUninitialize() // Uninitialize COM for this thread
}

// OllamaChatMessage defines the structure for a message in the Ollama API
// Renamed from OllamaMessage to avoid conflict if there was a global one.
// If OllamaMessage was already defined as this, then this definition is fine
type OllamaChatMessage struct { // Ensure this type is used consistently, or rename if it was OllamaMessage globally
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatRequest defines the structure for the Ollama API chat request
type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"` // Uses the locally defined OllamaChatMessage
	Stream   bool                `json:"stream"`
}

// OllamaChatResponse defines the structure for each chunk in the Ollama API stream
// Renamed from OllamaStreamResponse to avoid conflict if there was a global one.
type OllamaChatResponse struct { // Ensure this type is used consistently
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   OllamaChatMessage `json:"message"` // Uses the locally defined OllamaChatMessage
	Done      bool              `json:"done"`
}

// OllamaStreamEvent is the payload sent to the frontend for each stream event
type OllamaStreamEvent struct {
	Content        string  `json:"content,omitempty"`
	Done           bool    `json:"done"`
	Error          string  `json:"error,omitempty"`
	DurationMs     int64   `json:"durationMs,omitempty"`     // Total duration for the response in milliseconds
	RunesPerSecond float64 `json:"runesPerSecond,omitempty"` // Processed runes per second
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
		return nil, fmt.Errorf("ollama embeddings API error (%s): %s", resp.Status, string(responseBodyBytes))
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

// LoadPersonalData is a Wails-bindable method that prompts the user to select a directory,
// then processes .txt files from that directory, chunks them, generates embeddings, and stores them in memory.
func (a *App) LoadPersonalData() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Println("LoadPersonalData called. Prompting user to select a directory.")
	dialogOptions := runtime.OpenDialogOptions{
		Title:            "Select Folder Containing Your Documents",
		DefaultDirectory: "C:\\\\", // Set a simple, known valid default directory
		// ShowHiddenFiles: false, // Optional
	}

	directoryPath, err := runtime.OpenDirectoryDialog(a.ctx, dialogOptions)
	if err != nil {
		// Check if the error is the specific "shellItem is nil" which we treat as cancellation on Windows
		// This can happen if the user closes the dialog (e.g. with ESC or 'X')
		if err.Error() == "shellItem is nil" {
			statusMsg := "Document loading cancelled by user (dialog closed)."
			log.Println(statusMsg)
			return statusMsg // Return a user-friendly message
		}
		// For other errors, report them
		errMsg := fmt.Sprintf("error opening directory dialog: %v", err)
		log.Println(errMsg)
		return errMsg
	}

	if directoryPath == "" {
		// This case handles cancellation where err is nil but path is empty (e.g., user presses the "Cancel" button if available, or selects nothing and clicks "OK")
		statusMsg := "Document loading cancelled by user (no directory selected)."
		log.Println(statusMsg)
		return statusMsg
	}

	log.Printf("User selected directory: %s. Starting to load personal data.", directoryPath)

	// Clear existing document store and reset ID under lock
	a.documentStore = make([]DocumentChunk, 0)
	a.nextDocumentID = 1
	filesProcessed := 0
	chunksLoaded := 0

	dirEntries, err := os.ReadDir(directoryPath)
	if err != nil {
		errMsg := fmt.Sprintf("error reading directory %s: %v", directoryPath, err) // Changed: "Error" to "error"
		log.Println(errMsg)
		return errMsg
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".txt") {
			filePath := filepath.Join(directoryPath, entry.Name())
			log.Printf("Processing file: %s", filePath)

			content, err := os.ReadFile(filePath)
			if err != nil {
				log.Printf("Error reading file %s: %v. Skipping.", filePath, err)
				continue // Skip this file and continue with the next
			}

			// Define chunking parameters (could be made configurable later)
			chunkSizeWords := 200 // Example: 200 words per chunk
			overlapWords := 20    // Example: 20 words overlap

			textChunks := chunkText(string(content), chunkSizeWords, overlapWords)
			log.Printf("File %s split into %d chunks.", filePath, len(textChunks))

			for _, chunkText := range textChunks {
				embedding, err := a.getOllamaEmbedding(chunkText)
				if err != nil {
					log.Printf("Error getting embedding for a chunk from %s: %v. Skipping chunk.", filePath, err)
					continue // Skip this chunk
				}

				newChunk := DocumentChunk{
					ID:         a.nextDocumentID,
					Text:       chunkText,
					Embedding:  embedding,
					SourceFile: entry.Name(), // Store just the file name as source
				}
				a.documentStore = append(a.documentStore, newChunk)
				a.nextDocumentID++
				chunksLoaded++
			}
			filesProcessed++
		}
	}

	statusMessage := fmt.Sprintf("Successfully processed %d files, loaded %d chunks into document store from %s.", filesProcessed, chunksLoaded, directoryPath)
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
		// Assign the chunk and its calculated score to the result
		chunkWithScore := rankedChunks[i].chunk
		chunkWithScore.Score = rankedChunks[i].score // Explicitly set the score
		resultChunks[i] = chunkWithScore
		log.Printf("Selected relevant chunk %d: ID %d, Source: %s, Score: %.4f", i+1, resultChunks[i].ID, resultChunks[i].SourceFile, resultChunks[i].Score)
	}

	return resultChunks
}

// askOllamaChatRaw sends a request to Ollama's chat API and streams the response via Wails events.
// It should be run in a goroutine.
func (a *App) askOllamaChatRaw(messages []OllamaChatMessage) { // Changed parameter type to OllamaChatMessage
	startTime := time.Now()
	totalRunes := 0
	var finalErrorMessage string
	var accumulatedContent strings.Builder // Accumulate content here for the final event if needed, or just for rune counting

	// Ensure a final "done" event is sent when this function exits, regardless of path.
	defer func() {
		duration := time.Since(startTime)
		durationMs := duration.Milliseconds()
		runesPerSecond := 0.0
		if duration.Seconds() > 0 && totalRunes > 0 {
			runesPerSecond = float64(totalRunes) / duration.Seconds()
		}

		log.Printf("askOllamaChatRaw finished. Total runes: %d, Duration: %s (%.2f ms), Runes/s: %.2f. Sending final 'done' event.",
			totalRunes, duration, float64(durationMs), runesPerSecond)

		// The 'Content' field in this final 'done' event can be empty if all content was streamed progressively.
		// Or, if frontend prefers, accumulatedContent.String() could be sent here.
		// For now, individual chunks are sent, and this final event just signals completion and metrics.
		runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{
			Done:           true,
			Error:          finalErrorMessage,
			DurationMs:     durationMs,
			RunesPerSecond: runesPerSecond,
			// Content: accumulatedContent.String(), // Optionally send all content again, or leave for progressive updates
		})
	}()

	ollamaChatURL := ollamaApiUrl + "/chat" // Corrected URL construction
	requestPayload := OllamaChatRequest{
		Model:    chatModelName, // chatModelName is const
		Messages: messages,
		Stream:   true,
	}

	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		errMsg := fmt.Sprintf("error marshalling ollama request: %v", err)
		log.Println(errMsg)
		// a.sendErrorEvent(errMsg) // Deprecated
		finalErrorMessage = errMsg
		return // Defers will run, including the done event
	}

	log.Printf("Sending request to Ollama: %s", string(requestBody))

	// Use a.ctx for the request, so it can be cancelled if the app shuts down.
	req, err := http.NewRequestWithContext(a.ctx, "POST", ollamaChatURL, bytes.NewBuffer(requestBody))
	if err != nil {
		errMsg := fmt.Sprintf("error creating ollama request: %v", err)
		log.Println(errMsg)
		// a.sendErrorEvent(errMsg) // Deprecated
		finalErrorMessage = errMsg
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Check for context cancellation (e.g., app closing)
		if a.ctx.Err() != nil {
			log.Printf("Ollama request cancelled (context error): %v", a.ctx.Err())
			// No need to set finalErrorMessage if app is closing, defer will send done.
			// finalErrorMessage = fmt.Sprintf("Request cancelled: %v", a.ctx.Err()) // Or set it if you want to show this specific error
			return
		}
		errMsg := fmt.Sprintf("error sending request to ollama: %v", err)
		log.Println(errMsg)
		// a.sendErrorEvent(errMsg) // Deprecated
		finalErrorMessage = errMsg
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // Best effort to read body for error
		errMsg := fmt.Sprintf("ollama API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		log.Println(errMsg)
		// a.sendErrorEvent(errMsg) // Deprecated
		finalErrorMessage = errMsg
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	streamEndedByOllama := false // Flag to track if Ollama itself signaled completion
	for scanner.Scan() {
		// Check context in loop to allow for cancellation during long streams
		if a.ctx.Err() != nil {
			log.Printf("Stream processing cancelled (context error): %v", a.ctx.Err())
			// finalErrorMessage = fmt.Sprintf("Stream cancelled: %v", a.ctx.Err()) // Optional: set error message
			return // Exit, defer will send final done event
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ollamaResp OllamaChatResponse // Changed to use OllamaChatResponse
		if err := json.Unmarshal(line, &ollamaResp); err != nil {
			errMsg := fmt.Sprintf("error unmarshalling stream response: %v. Line: %s", err, string(line))
			log.Println(errMsg)
			// a.sendErrorEvent(errMsg) // Send error for this chunk // Deprecated
			// If unmarshal fails, we might consider the stream corrupted and stop.
			finalErrorMessage = errMsg // Set the error and let defer handle it.
			return                     // Stop processing stream.
		}

		if ollamaResp.Message.Content != "" {
			totalRunes += utf8.RuneCountInString(ollamaResp.Message.Content)
			accumulatedContent.WriteString(ollamaResp.Message.Content)        // Keep accumulating for accurate total rune count
			runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{ // Use struct
				Content: ollamaResp.Message.Content,
				Done:    false, // This is an intermediate chunk
			})
		}

		if ollamaResp.Done { // This is the Done flag from the Ollama stream chunk itself
			log.Println("Ollama stream chunk indicates 'done'. Loop will terminate.")
			streamEndedByOllama = true
			break // Exit the scanner loop; the deferred function will send the final done event.
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		// Don't send error if context was cancelled, as that's the primary error reason.
		if a.ctx.Err() == nil {
			errMsg := fmt.Sprintf("error reading stream response: %v", err)
			log.Println(errMsg)
			// a.sendErrorEvent(errMsg) // Deprecated
			if finalErrorMessage == "" { // Only set if not already set by a more specific error
				finalErrorMessage = errMsg
			}
		}
		// Defer will handle sending the final event
		return
	}

	if !streamEndedByOllama && finalErrorMessage == "" {
		log.Println("Stream ended without Ollama signaling 'done' and no prior errors. This is unusual but handled.")
		// The defer function will still send a 'done' event with metrics.
	}
	// Normal exit: defer function sends the final done event with metrics.
}

// HandleMessage is called when the user sends a message.
// It processes the input, performs RAG, and triggers AI response streaming.
func (a *App) HandleMessage(userInput string) error {
	log.Printf("HandleMessage received: %s", userInput)

	// 1. Get embedding for the user input
	queryEmbedding, err := a.getOllamaEmbedding(userInput)
	if err != nil {
		errMsg := fmt.Sprintf("Error getting embedding for your message: %v", err)
		log.Println(errMsg)
		// Send an error event to the frontend immediately
		runtime.EventsEmit(a.ctx, "ollamaStreamEvent", OllamaStreamEvent{
			Error: errMsg,
			Done:  true, // Signal completion of this attempt
		})
		return fmt.Errorf("%s", errMsg) // Also return error to Wails caller
	}

	// 2. Find relevant chunks
	topN := 3 // Number of relevant chunks to retrieve
	log.Printf("Finding top %d relevant chunks for input: '%s'", topN, userInput)
	relevantChunks := a.findRelevantChunks(queryEmbedding, topN)

	// Prepare source information for the frontend
	sourceInfos := make([]SourceInfo, 0, len(relevantChunks))
	for _, chunk := range relevantChunks {
		sourceInfos = append(sourceInfos, SourceInfo{
			FileName: chunk.SourceFile,
			ChunkID:  chunk.ID,
			Score:    chunk.Score,
		})
	}

	// Emit an event with RAG sources *before* starting the AI response stream
	// This allows the UI to display sources immediately.
	if len(sourceInfos) > 0 {
		log.Printf("Emitting %d RAG sources via 'ragSourcesEvent'", len(sourceInfos))
		runtime.EventsEmit(a.ctx, "ragSourcesEvent", sourceInfos)
	} else {
		log.Println("No RAG sources found to emit.")
	}

	// 3. Construct context from relevant chunks
	var contextBuilder strings.Builder
	if len(relevantChunks) > 0 {
		contextBuilder.WriteString("Use the following context to answer the user's question:\n\n")
		for i, chunk := range relevantChunks {
			contextBuilder.WriteString(fmt.Sprintf("Context from document '%s' (Chunk %d, Relevance: %.2f):\n", chunk.SourceFile, chunk.ID, chunk.Score))
			contextBuilder.WriteString(chunk.Text)
			if i < len(relevantChunks)-1 {
				contextBuilder.WriteString("\n\n---\n\n") // Separator between chunks
			} else {
				contextBuilder.WriteString("\n\n")
			}
		}
		log.Printf("Constructed RAG context (length: %d chars)", contextBuilder.Len())
	} else {
		log.Println("No relevant chunks found or RAG context is empty.")
	}

	// 4. Formulate the prompt for the LLM
	var llmPrompt string
	if contextBuilder.Len() > 0 {
		llmPrompt = contextBuilder.String() + "User's question: " + userInput
	} else {
		llmPrompt = userInput // No RAG context, just send the raw user input
	}

	// 5. Prepare messages for Ollama
	messages := []OllamaChatMessage{
		// Optional: Add a system prompt here if desired
		// {Role: "system", Content: "You are a helpful assistant. Please use the provided context to answer the user's question. If the context is not relevant, say so."},
		{Role: "user", Content: llmPrompt},
	}

	// 6. Call askOllamaChatRaw in a goroutine to handle streaming
	log.Printf("Calling askOllamaChatRaw with LLM prompt (first 100 chars of user content): %s...", llmPrompt[:min(len(llmPrompt), 100)])
	go a.askOllamaChatRaw(messages)

	return nil // Indicate success for the synchronous part of HandleMessage
}
