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
      - Constructs a request for Ollama\'s `/api/chat` endpoint (using the chat model, e.g., `llama3`, and `stream: false`).
      - Makes an HTTP POST request to `http://localhost:11434/api/chat`.
      - Parses the response and returns the AI\'s message content.
      - Includes basic error handling and logging.
      - [x] (Optional) Modify to support streaming responses from Ollama (`stream: true`).
  - **`frontend/src/App.tsx` (or equivalent):**
    - [x] Create state for input text and messages array.
    - [x] Create an input field for the user to type messages.
    - [x] Create a \"Send\" button.
    - [x] On send, call the Go `HandleMessage` function via `window.go.main.App.HandleMessage(inputText)`.
    - [x] Add the user\'s message and the AI\'s response to the messages array.
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
  - [x] Define constants/variables for `embeddingModelName` (\"nomic-embed-text\"), `chatModelName` (\"llama3\"), and `ollamaApiUrl` (\"http://localhost:11434\").
- [x] **Implement Ollama API Call for Embeddings:**
  - [x] Create `getOllamaEmbedding(text string) ([]float64, error)` function:
    - Constructs a request for Ollama\'s `/api/embeddings` endpoint using `embeddingModelName`.
    - Makes an HTTP POST request.
    - Parses the response and returns the embedding vector or an error.
- [x] **Implement Document Processing:**
  - [x] Create `chunkText(text string, chunkSize int, overlap int) []string` helper function (e.g., split by words, configurable size/overlap).
  - [x] Create `LoadPersonalData(directoryPath string) string` Wails-bindable method in `App` struct:
    - [x] Clears `documentStore`.
    - [x] Reads all `.txt` files from the provided `directoryPath`.
    - [x] For each file\'s content:
      - [x] Use `chunkText` to split it into manageable chunks.
      - [x] For each chunk:
        - [x] Call `getOllamaEmbedding` to get its embedding.
        - [x] Create a `DocumentChunk` object and add it to `documentStore`.
        - [x] Increment `nextDocumentID`.
    - [x] Return a status message (e.g., \"Processed X files, Y chunks loaded.\").
    - [x] Add logging for progress and errors.
- [x] **Implement Similarity Search:**
  - [x] Create `cosineSimilarity(vecA, vecB []float64) (float64, error)` function.
  - [x] Create `findRelevantChunks(queryEmbedding []float64, topN int) []DocumentChunk` function:
    - [x] Iterates through `documentStore`.
    - [x] Calculates cosine similarity between `queryEmbedding` and each chunk\'s embedding.
    - [x] Sorts chunks by similarity score (descending).
    - [x] Returns
- [x] **Update `HandleMessage(userInput string) string` for RAG:**
  - [x] **Check `documentStore`:** If empty, log a message and proceed with a non-RAG call to `askOllamaChatRaw`.
  - [x] **Get Query Embedding:** Call `getOllamaEmbedding` for the `userInput`. Handle errors by emitting an event.
  - [x] **Find Relevant Chunks:** Call `findRelevantChunks` (e.g., `topN = 3`).
  - [x] **Check Relevant Chunks:** If no relevant chunks are found, log a message and proceed with a non-RAG call.
  - [x] **Construct Augmented Prompt:**
    - Start with a preamble.
    - Append the text of each relevant chunk.
    - Append the user\'s original question.
    - Log the full augmented prompt.
  - [x] **Call Chat LLM:**
    - Create a message list with the (potentially augmented) prompt.
    - Call `askOllamaChatRaw` with the messages.
    - Handle errors (within `askOllamaChatRaw` by emitting events).
  - [x] Return the LLM\'s response (empty string, actual response via events).

---

## Phase 4: Frontend Integration & Backend Adjustments

- [x] **UI for Loading Personal Data:**
  - [x] **`frontend/src/App.tsx`:**
    - [x] Add a \"Load Data\" button.
    - [x] On click, use Wails `window.runtime.DialogOpenFile` or `DialogSelectFolder` to let the user pick a directory.
    - [x] Call Go `LoadPersonalData(selectedPath)` with the chosen path.
    - [x] Display the status message returned from Go.
    - [x] Add loading/disabled states for the button during data processing.
- [x] **Refine `HandleMessage` and `LoadPersonalData` in `app.go`:**
  - [x] Ensure `HandleMessage` uses streaming and emits events for AI responses (`OllamaStreamEvent` with `type: \"response\"` or `type: \"error\"`).
  - [x] Ensure `LoadPersonalData` emits events for progress/completion/errors (`DataLoadEvent` with `status`, `message`, `error`).
  - [x] Implement robust error handling and logging in both.
  - [x] Add mutexes to protect shared data like `documentStore` if accessed by concurrent goroutines.
- [x] **Update Frontend to Handle New Events:**
  - [x] **`frontend/src/App.tsx`:**
    - [x] Listen for `OllamaStreamEvent` and update chat messages.
    - [x] Listen for `DataLoadEvent` and display feedback to the user (e.g., loading indicators, success/error messages).
- [x] **Test RAG Workflow:**
  - [x] Load some `.txt` files.
  - [x] Ask questions that should trigger RAG.
  - [x] Verify that the responses seem to incorporate information from the loaded documents.
  - [x] Check Go logs for embedding generation, chunk retrieval, and augmented prompt.
- [x] **(Optional) Display Context Source:**
  - [x] **`app.go`:**
    - [x] Define `SourceInfo struct { FileName string; ChunkID int; Score float64 }`.
    - [x] In `HandleMessage`, when RAG is used, populate a `[]SourceInfo` from the `relevantChunks`.
    - [x] Emit a new Wails event (e.g., `ragContextSources`) with this `[]SourceInfo` _before_ calling the LLM for the final answer. If RAG is skipped or fails, emit an empty slice.
  - [x] **`frontend/src/App.tsx`:**
    - [x] Define a corresponding `SourceInfo` interface.
    - [x] Add state to store `SourceInfo[]`.
    - [x] Listen for the `ragContextSources` event and update the state.
    - [x] Clear sources when a new message is sent or new data is loaded.
    - [x] Display the source information (e.g., filename, chunk ID, score) in the UI, perhaps below the relevant AI message or in a dedicated section.
  - [x] **`frontend/src/App.css` (or equivalent):**
    - [x] Add basic styling for the context source display.
- [x] **Review and Address \"No listeners for event\"**:
  - [x] Ensure all Go event emissions (`runtime.EventsEmit`) have corresponding `runtime.EventsOn` listeners in the frontend for the _exact_ event names.
  - [x] Verify event names are consistent between backend and frontend.
  - [x] If issues persist, simplify event handling to isolate the problem.
  - [x] Concluded that remaining \"TRA | No listeners...\" messages are likely benign dev environment noise as UI functions correctly.
  - [x] Attempted to reduce Wails dev console verbosity by setting `LogLevel: logger.DEBUG` in `main.go`.

---

## Phase 5: Testing & Refinement

- [x] **Render AI responses as Markdown:**
  - [x] Install `react-markdown` and `remark-gfm` in the frontend.
  - [x] Update `App.tsx` to use `ReactMarkdown` component for AI messages.
  - [x] Configure Vite to handle `react-markdown` dependencies if necessary (e.g., `external` in `vite.config.ts`).
- [x] **Display AI response generation time and speed:**
  - [x] Backend: Modify `askOllamaChatRaw` in `app.go` to calculate and return duration (ms) and speed (runes/sec) in the final `OllamaStreamEvent`.
  - [x] Frontend: Update `Message` and `OllamaStreamEventPayload` interfaces in `App.tsx`.
  - [x] Frontend: Adjust `ollamaStreamEvent` listener in `App.tsx` to store metrics from the final event.
  - [x] Frontend: Render metrics (duration, speed, error indicator) below AI messages in `App.tsx`.
  - [x] Frontend: Add CSS for `.ai-message-metrics`, `.ai-message-extras`, and `.error-indicator` in `App.css`.
- [ ] **Comprehensive Testing:**
  - [ ] Test RAG source display thoroughly.
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
  - Run the executable on a Windows machine (ideally one that didn\'t have the dev environment).
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
