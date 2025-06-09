# Progress: Offline RAG Chat Application (Wails + Ollama on Windows)

**Project Goal:** Build a desktop application using Wails (Go + React/TS frontend) that allows users to chat with an LLM (via Ollama) using Retrieval Augmented Generation (RAG) from their personal documents. The application should run entirely offline on Windows once Ollama and models are set up.

---

## Phase 1: Environment Setup (Windows)

- [x] **Install Ollama for Windows:**
  - Download from [ollama.com](https://ollama.com/).
  - Follow installation instructions.
- [x] **Verify Ollama Installation:**
  - Run `ollama --version` in terminal.
- [x] **Pull Necessary Ollama Models:**
  - [x] Chat model: `ollama pull llama3` (or your preferred chat model).
  - [x] Embedding model: `ollama pull nomic-embed-text`.
- [x] **Confirm Ollama Service and Models:**
  - Ensure Ollama service is running (usually starts automatically).
  - List available models: `ollama list`.

---

## Phase 2: Project Initialization & Basic Chat

- [x] **Create or Clone Wails Project:**
  - If new: `wails init -n medical-awp -t react-ts` (replace `react-ts` with your preferred template if different).
  - If cloning an existing Wails project structure: `git clone <your-repository-url>`.
  - Navigate into the project directory: `cd medical-awp`.
- [x] **Implement Basic Ollama Chat Functionality (Non-RAG):**
  - **`app.go`:**
    - [x] Create an `App` struct.
    - [x] Implement `NewApp()` and `Startup()` methods.
    - [x] Implement a `HandleMessage(userInput string) string` method that:
      - Takes user input.
      - Constructs a request for Ollama's `/api/chat` endpoint (using the chat model, e.g., `llama3`, and `stream: false`).
      - Makes an HTTP POST request to `http://localhost:11434/api/chat`.
      - Parses the response and returns the AI's message content.
      - Includes basic error handling and logging.
      - [x] (Optional) Modify to support streaming responses from Ollama (`stream: true`).
  - **`frontend/src/App.tsx` (or equivalent):**
    - [x] Create state for input text and messages array.
    - [x] Create an input field for the user to type messages.
    - [x] Create a "Send" button.
    - [x] On send, call the Go `HandleMessage` function via `window.go.main.App.HandleMessage(inputText)`.
    - [x] Add the user's message and the AI's response to the messages array.
    - [x] Render the messages array in a chat-like interface.
    - [x] (Optional) Implement a loading indicator while waiting for AI response.
    - [x] (Optional) Update to handle streaming AI responses if implemented in backend.
- [x] **Test Basic Chat:**
  - Run `wails dev`.
  - Ensure you can send messages and receive responses from Ollama.

---

## Phase 3: RAG Backend Implementation (`app.go`)

- [x] **Define RAG Data Structures:**
  - [x] `DocumentChunk` struct: `ID int`, `Text string`, `Embedding []float64`, `SourceFile string`.
  - [x] Global or App-level variable: `documentStore []DocumentChunk` (for in-memory storage).
  - [x] Global or App-level variable: `nextDocumentID int`.
  - [x] Define constants/variables for `embeddingModelName` ("nomic-embed-text"), `chatModelName` ("llama3"), and `ollamaApiUrl` ("http://localhost:11434").
- [x] **Implement Ollama API Call for Embeddings:**
  - [x] Create `getOllamaEmbedding(text string) ([]float64, error)` function:
    - Constructs a request for Ollama's `/api/embeddings` endpoint using `embeddingModelName`.
    - Makes an HTTP POST request.
    - Parses the response and returns the embedding vector or an error.
- [x] **Implement Document Processing:**
  - [x] Create `chunkText(text string, chunkSize int, overlap int) []string` helper function (e.g., split by words, configurable size/overlap).
  - [x] Create `LoadPersonalData(directoryPath string) string` Wails-bindable method in `App` struct:
    - [x] Clears `documentStore`.
    - [x] Reads all `.txt` files from the provided `directoryPath`.
    - [x] For each file's content:
      - [x] Use `chunkText` to split it into manageable chunks.
      - [x] For each chunk:
        - [x] Call `getOllamaEmbedding` to get its embedding.
        - [x] Create a `DocumentChunk` object and add it to `documentStore`.
        - [x] Increment `nextDocumentID`.
    - [x] Return a status message (e.g., "Processed X files, Y chunks loaded.").
    - [x] Add logging for progress and errors.
- [x] **Implement Similarity Search:**
  - [x] Create `cosineSimilarity(vecA, vecB []float64) (float64, error)` function.
  - [x] Create `findRelevantChunks(queryEmbedding []float64, topN int) []DocumentChunk` function:
    - [x] Iterates through `documentStore`.
    - [x] Calculates cosine similarity between `queryEmbedding` and each chunk's embedding.
    - [x] Sorts chunks by similarity score (descending).
    - [x] Returns the top `N` most similar `DocumentChunk`s.
- [x] **Update `HandleMessage(userInput string) string` for RAG:**
  - [x] **Check `documentStore`:** If empty, log a message and proceed with a non-RAG call to `askOllamaChatRaw`.
  - [x] **Get Query Embedding:** Call `getOllamaEmbedding` for the `userInput`. Handle errors by emitting an event.
  - [x] **Find Relevant Chunks:** Call `findRelevantChunks` (e.g., `topN = 3`).
  - [x] **Check Relevant Chunks:** If no relevant chunks are found, log a message and proceed with a non-RAG call.
  - [x] **Construct Augmented Prompt:**
    - Start with a preamble.
    - Append the text of each relevant chunk.
    - Append the user's original question.
    - Log the full augmented prompt.
  - [x] **Call Chat LLM:**
    - Create a message list with the (potentially augmented) prompt.
    - Call `askOllamaChatRaw` with the messages.
    - Handle errors (within `askOllamaChatRaw` by emitting events).
  - [x] Return the LLM's response (empty string, actual response via events).

---

## Phase 4: Frontend Integration (`App.tsx`)

- [x] **Add UI for Loading Personal Data:**
  - [x] Add a "Load Personal Data" button.
  - [x] On button click, use `window.runtime.OpenDirectoryDialog()` to let the user select a folder.
  - [x] If a folder is selected, call the Go `LoadPersonalData` method.
  - [x] Display a status message in the UI (e.g., "Loading...", "X documents loaded", "Error: ...").
- [x] **Verify Chat Interface:**
  - [x] Ensure the chat still works as expected after the RAG backend changes.
  - [x] Test sending messages with and without personal data loaded.
- [ ] **(Optional) Display Context Source:**
  - [ ] If RAG is used, consider displaying which document chunks were used as context for the AI's response.
    - This might involve modifying the `ollamaStreamEvent` or adding a new event to pass source information.
    - Update the UI to show this information, perhaps subtly or on hover/click.

---

## Phase 5: Testing & Refinement

- [ ] **Comprehensive Testing:**
  - [ ] Test with various `.txt` files (empty, large, different encodings if applicable).
  - [ ] Test with different folder structures.
  - [ ] Test edge cases for chunking and embedding.
  - [ ] Test error handling for Ollama API calls (e.g., Ollama service down, model not available).
  - [ ] Test UI responsiveness and error display.
- [ ] **Refine Chunking Strategy:**
  - [ ] Experiment with `chunkSize` and `overlap` for optimal RAG performance.
  - [ ] Consider more advanced chunking methods if needed (e.g., sentence splitting).
- [ ] **Refine Prompt Engineering:**
  - [ ] Optimize the augmented prompt structure for clarity and effectiveness.
- [ ] **Improve Error Handling and User Feedback:**
  - [ ] Ensure all errors are caught gracefully and informative messages are shown to the user.
- [ ] **Code Cleanup and Optimization:**
  - [ ] Refactor code for readability and maintainability.
  - [ ] Optimize performance where necessary.

---

## Phase 6: Build for Distribution (Windows)

- [ ] **Build the Application:**
  - Run `wails build`.
  - This will create an executable in the `build/bin` directory.
- [ ] **Test the Standalone Build:**
  - Run the executable on a Windows machine (ideally one that didn't have the dev environment).
  - Ensure Ollama (and its models) are set up on the test machine as a prerequisite.
- [ ] **(Optional) Create an Installer:**
  - Consider using a tool like NSIS or Inno Setup to create an installer for easier distribution.
- [ ] **Document Prerequisites:**
  - Clearly document that users need to have Ollama installed and the required models (`llama3`, `nomic-embed-text`) pulled.

---

## Future Enhancements (Post-MVP)

- [ ] Support for more document types (e.g., `.pdf`, `.docx`) using Go libraries.
- [ ] Persistent document store (e.g., using SQLite or a simple file-based database) instead of in-memory.
- [ ] UI for managing loaded documents (e.g., view list, remove documents).
- [ ] Option to select different Ollama models from the UI.
- [ ] More sophisticated RAG techniques (e.g., re-ranking, query transformations).
- [ ] Cross-platform builds (macOS, Linux) if desired.
