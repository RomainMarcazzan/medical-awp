# Progress: Offline RAG Chat Application (Wails + Ollama on Windows)

**Project Goal:** Build a desktop application using Wails (Go + React/TS frontend) that allows users to chat with an LLM (via Ollama) using Retrieval Augmented Generation (RAG) from their personal documents. The application should run entirely offline on Windows once Ollama and models are set up.

---

## Phase 1: Environment Setup (Windows)

- [ ] **Install Ollama for Windows:**
  - Download from [ollama.com](https://ollama.com/).
  - Follow installation instructions.
- [ ] **Verify Ollama Installation:**
  - Run `ollama --version` in terminal.
- [ ] **Pull Necessary Ollama Models:**
  - [ ] Chat model: `ollama pull llama3` (or your preferred chat model).
  - [ ] Embedding model: `ollama pull nomic-embed-text`.
- [ ] **Confirm Ollama Service and Models:**
  - Ensure Ollama service is running (usually starts automatically).
  - List available models: `ollama list`.

---

## Phase 2: Project Initialization & Basic Chat

- [ ] **Create or Clone Wails Project:**
  - If new: `wails init -n medical-awp-rag -t react-ts` (replace `react-ts` with your preferred template if different).
  - If cloning an existing Wails project structure: `git clone <your-repository-url>`.
  - Navigate into the project directory: `cd medical-awp-rag`.
- [ ] **Implement Basic Ollama Chat Functionality (Non-RAG):**
  - **`app.go`:**
    - [ ] Create an `App` struct.
    - [ ] Implement `NewApp()` and `Startup()` methods.
    - [ ] Implement a `HandleMessage(userInput string) string` method that:
      - Takes user input.
      - Constructs a request for Ollama's `/api/chat` endpoint (using the chat model, e.g., `llama3`, and `stream: false`).
      - Makes an HTTP POST request to `http://localhost:11434/api/chat`.
      - Parses the response and returns the AI's message content.
      - Includes basic error handling and logging.
  - **`frontend/src/App.tsx` (or equivalent):**
    - [ ] Create state for input text and messages array.
    - [ ] Create an input field for the user to type messages.
    - [ ] Create a "Send" button.
    - [ ] On send, call the Go `HandleMessage` function via `window.go.main.App.HandleMessage(inputText)`.
    - [ ] Add the user's message and the AI's response to the messages array.
    - [ ] Render the messages array in a chat-like interface.
- [ ] **Test Basic Chat:**
  - Run `wails dev`.
  - Ensure you can send messages and receive responses from Ollama.

---

## Phase 3: RAG Backend Implementation (`app.go`)

- [ ] **Define RAG Data Structures:**
  - [ ] `DocumentChunk` struct: `ID int`, `Text string`, `Embedding []float64`, `SourceFile string`.
  - [ ] Global or App-level variable: `documentStore []DocumentChunk` (for in-memory storage).
  - [ ] Global or App-level variable: `nextDocumentID int`.
  - [ ] Define constants/variables for `embeddingModelName` ("nomic-embed-text"), `chatModelName` ("llama3"), and `ollamaApiUrl` ("http://localhost:11434").
- [ ] **Implement Ollama API Call for Embeddings:**
  - [ ] Create `getOllamaEmbedding(text string) ([]float64, error)` function:
    - Constructs a request for Ollama's `/api/embeddings` endpoint using `embeddingModelName`.
    - Makes an HTTP POST request.
    - Parses the response and returns the embedding vector or an error.
- [ ] **Implement Document Processing:**
  - [ ] Create `chunkText(text string, chunkSize int, overlap int) []string` helper function (e.g., split by words, configurable size/overlap).
  - [ ] Create `LoadPersonalData(directoryPath string) string` Wails-bindable method in `App` struct:
    - [ ] Clears `documentStore`.
    - [ ] Reads all `.txt` files from the provided `directoryPath`.
    - [ ] For each file's content:
      - [ ] Use `chunkText` to split it into manageable chunks.
      - [ ] For each chunk:
        - [ ] Call `getOllamaEmbedding` to get its embedding.
        - [ ] Create a `DocumentChunk` object and add it to `documentStore`.
        - [ ] Increment `nextDocumentID`.
    - [ ] Return a status message (e.g., "Processed X files, Y chunks loaded.").
    - [ ] Add logging for progress and errors.
- [ ] **Implement Similarity Search:**
  - [ ] Create `cosineSimilarity(vecA, vecB []float64) (float64, error)` function.
  - [ ] Create `findRelevantChunks(queryEmbedding []float64, topN int) []DocumentChunk` function:
    - [ ] Iterates through `documentStore`.
    - [ ] Calculates cosine similarity between `queryEmbedding` and each chunk's embedding.
    - [ ] Sorts chunks by similarity score (descending).
    - [ ] Returns the top `N` most similar `DocumentChunk`s.
- [ ] **Update `HandleMessage(userInput string) string` for RAG:**
  - [ ] **Check `documentStore`:** If empty, log a message and proceed with a non-RAG call to `askOllamaChat` (from Phase 2, possibly refactored into its own function).
  - [ ] **Get Query Embedding:** Call `getOllamaEmbedding` for the `userInput`. Handle errors.
  - [ ] **Find Relevant Chunks:** Call `findRelevantChunks` (e.g., `topN = 3`).
  - [ ] **Check Relevant Chunks:** If no relevant chunks are found, log a message and proceed with a non-RAG call.
  - [ ] **Construct Augmented Prompt:**
    - Start with a preamble (e.g., "Based on the following information from your documents:").
    - Append the text of each relevant chunk (e.g., "--- Context from [SourceFile] ---\n[Chunk.Text]\n").
    - Append the user's original question (e.g., "\n--- End of Context ---\n\nPlease answer this question: " + `userInput`).
    - Log the full augmented prompt.
  - [ ] **Call Chat LLM:**
    - Create a message list (e.g., `[]OllamaChatMessage{{Role: "user", Content: augmentedPrompt}}`).
    - Call a refactored `askOllamaChat(messages []OllamaChatMessage) (string, error)` function with the augmented prompt.
    - Handle errors.
  - [ ] Return the LLM's response.
- [ ] **Add Logging:** Ensure comprehensive logging throughout the RAG process (data loading, embedding generation, chunk retrieval, prompt construction, API calls).

---

## Phase 4: Frontend Integration (`App.tsx` or equivalent)

- [ ] **Add UI for Loading Personal Data:**
  - [ ] Add a "Load Personal Data" button.
  - [ ] On button click, use `window.runtime.OpenDirectoryDialog()` to let the user select a folder.
  - [ ] If a folder is selected, call the Go `LoadPersonalData` method via `window.go.main.App.LoadPersonalData(selectedPath)`.
  - [ ] Display a status message (e.g., "Loading data...", or the success/error message returned from Go) in the UI.
- [ ] **Verify Chat Interface:**
  - Ensure the existing chat input and message display still function correctly with the updated `HandleMessage` backend.
- [ ] **(Optional) Display Context Source:**
  - Consider if/how you might indicate to the user that the answer was derived from their documents (e.g., by listing source files if the Go backend can provide this info with the response).

---

## Phase 5: Testing & Refinement (Windows)

- [ ] **Prepare Test Data:** Create a folder on your Windows machine with several `.txt` files containing diverse content.
- [ ] **Run `wails dev`**.
- [ ] **Test `LoadPersonalData`:**
  - [ ] Click the "Load Personal Data" button and select your test folder.
  - [ ] Check the Wails console (terminal output) for logs from `app.go` regarding file processing, chunking, and embedding.
  - [ ] Verify the status message displayed in the UI.
- [ ] **Test RAG Queries:**
  - [ ] Ask questions specifically related to the content of your test documents.
  - [ ] Check console logs to see:
    - The query embedding being generated.
    - Which chunks are identified as relevant.
    - The full augmented prompt sent to Ollama.
  - [ ] Evaluate the quality and relevance of the LLM's responses.
- [ ] **Test Fallback Behavior:**
  - [ ] Ask questions _before_ loading any personal data. Verify it uses the general chat mode.
  - [ ] Ask questions unrelated to your loaded documents. Verify it either uses general chat mode or indicates no relevant context was found.
- [ ] **Test Error Handling:**
  - [ ] Try running the app with Ollama service stopped.
  - [ ] Try querying with a model name that hasn't been pulled in Ollama.
  - [ ] Try loading data from an empty or non-existent directory.
- [ ] **Debug:** Address any bugs or unexpected behavior in both the Go backend and React frontend.
- [ ] **Iterate:** Refine chunking strategy, `topN` for relevant chunks, and prompt engineering for better results.

---

## Phase 6: Build for Distribution (Windows)

- [ ] **Build the Application:**
  - Run `wails build` in the project root. This will create an `.exe` file in the `build/bin` directory.
- [ ] **Test the Executable:**
  - Run the generated `.exe` file directly.
  - Retest core functionalities (loading data, RAG chat).
- [ ] **Document Prerequisites for Users:**
  - Clearly state that users need to have Ollama for Windows installed.
  - Specify which Ollama models need to be pulled (e.g., `ollama pull llama3`, `ollama pull nomic-embed-text`).

---

## Future Enhancements (Optional - Post MVP)

- [ ] Support more file types (e.g., PDF, DOCX) by integrating Go libraries for parsing them.
- [ ] Implement a persistent vector database (e.g., LanceDB, ChromaDB via Go bindings) instead of the in-memory `documentStore` for larger datasets and persistence across sessions.
- [ ] Improve text chunking strategy (e.g., semantic chunking, sentence-aware splitting).
- [ ] More sophisticated prompt engineering for RAG.
- [ ] Add a UI section for users to manage loaded data sources (e.g., view loaded files, re-index).
- [ ] Implement streaming responses from Ollama for a more interactive chat experience.
- [ ] Add settings for users to configure Ollama model names or API endpoint if needed.

---

**Notes:**

- Remember to handle errors gracefully at each step and provide feedback to the user.
- Logging in the Go backend (visible in the `wails dev` console) will be crucial for debugging.
- This plan assumes a basic in-memory RAG. For production or large datasets, a proper vector database is recommended.
