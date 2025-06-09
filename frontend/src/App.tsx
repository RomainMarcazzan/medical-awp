import { useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import "./App.css";
import { HandleMessage, LoadPersonalData } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime"; // Corrected import path for EventsOn

interface Message {
  id: number;
  text: string;
  sender: "user" | "ai";
  durationMs?: number;
  runesPerSecond?: number;
  isError?: boolean;
  sources?: SourceInfo[]; // Added to store sources directly with the AI message
}

// Define the structure of the event payload from Go
interface OllamaStreamEventPayload {
  content?: string; // content can be empty, especially in the final 'done' message
  done: boolean;
  error?: string;
  durationMs?: number;
  runesPerSecond?: number;
}

// Define the SourceInfo interface to match the Go struct
interface SourceInfo {
  fileName: string;
  chunkId: number;
  score: number;
}

function App() {
  const [input, setInput] = useState<string>("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [dataLoadStatus, setDataLoadStatus] = useState<string>("");
  const [isLoading, setIsLoading] = useState(false); // Loading state for chat messages
  const [isDataLoading, setIsDataLoading] = useState(false); // Loading state for personal data
  const [dataLoadingStatus, setDataLoadingStatus] = useState<string>(""); // Status message for data loading
  const [ragSources, setRagSources] = useState<SourceInfo[]>([]); // State for RAG sources
  const currentAiMessageIdRef = useRef<number | null>(null); // To track the ID of the AI message being streamed
  const messageEndRef = useRef<null | HTMLDivElement>(null);

  const scrollToBottom = () => {
    messageEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(scrollToBottom, [messages]);

  // Listen for streaming events from Go
  useEffect(() => {
    console.log("JS: App component mounted. Attempting to register ollamaStreamEvent listener.");

    // Updated interface to match observed event data - This local interface is not strictly necessary
    // if OllamaStreamEventPayload is correctly defined and used above.
    /*
    interface OllamaStreamEventData {
      content?: string; // Content is present for chunks
      done: boolean;    // True when the stream is complete
    }
    */

    const unlistenOllama = EventsOn("ollamaStreamEvent", (eventData: OllamaStreamEventPayload) => {
      console.log("JS: ollamaStreamEvent received in listener:", JSON.stringify(eventData));

      if (eventData.done) {
        // Only process "done" if we are still expecting it for a specific message
        if (currentAiMessageIdRef.current !== null) {
          const messageIdToUpdate = currentAiMessageIdRef.current;
          // Nullify the ref *before* the setMessages call for this stream's "done" event.
          // This makes the "done" processing for a given stream idempotent.
          currentAiMessageIdRef.current = null;

          setMessages((prevMessages) => {
            console.log(
              `JS: Processing 'done' event for message ID: ${messageIdToUpdate}. Data:`,
              JSON.stringify(eventData)
            );
            setIsLoading(false); // Also set isLoading to false here
            const newMessages = [...prevMessages];
            const aiMessageIndex = newMessages.findIndex((msg) => msg.id === messageIdToUpdate);

            if (aiMessageIndex !== -1) {
              newMessages[aiMessageIndex] = {
                ...newMessages[aiMessageIndex],
                durationMs: eventData.durationMs,
                runesPerSecond: eventData.runesPerSecond,
                isError: !!eventData.error,
              };
              if (eventData.error && newMessages[aiMessageIndex].text.length === 0) {
                newMessages[aiMessageIndex].text = `[Error: ${eventData.error}]`;
              } else if (eventData.error) {
                newMessages[aiMessageIndex].text += `\\\\n[Error: ${eventData.error}]`;
              }
              console.log("JS: AI Message updated with metrics:", JSON.stringify(newMessages[aiMessageIndex]));
            } else {
              // This case should be less likely now with the outer check, but good for robustness
              console.warn(
                `JS: 'done' event (for ID ${messageIdToUpdate}) processed, but no matching AI message found in prevMessages.`
              );
            }
            return newMessages;
          });
        } else {
          console.warn(
            "JS: 'done' event received, but currentAiMessageIdRef was already null (likely a duplicate or late event). Data:",
            JSON.stringify(eventData)
          );
        }
      } else if (typeof eventData.content === "string") {
        // Process intermediate content chunks only if there's an active AI message stream
        if (currentAiMessageIdRef.current !== null) {
          setMessages((prevMessages) => {
            const newMessages = [...prevMessages];
            const aiMessageIndex = newMessages.findIndex((msg) => msg.id === currentAiMessageIdRef.current);
            if (aiMessageIndex !== -1) {
              newMessages[aiMessageIndex] = {
                ...newMessages[aiMessageIndex],
                text: newMessages[aiMessageIndex].text + eventData.content,
              };
            } else {
              // This might happen if a content chunk arrives after its stream's "done" event was processed (due to async nature)
              console.warn(
                "JS: Intermediate stream data received, but no matching AI message found (currentAiMessageIdRef might have been nulled). ID ref:",
                currentAiMessageIdRef.current, // This will be null if "done" was processed
                "Event Content:",
                eventData.content
              );
            }
            return newMessages;
          });
        } else {
          console.warn(
            "JS: Intermediate stream data received, but no active AI message (currentAiMessageIdRef is null). Discarding. Data:",
            JSON.stringify(eventData)
          );
        }
      } else if (eventData.content !== undefined) {
        // Content is present but not a string, and not a "done" event
        console.warn("JS: Received event with non-string content (and not done):", JSON.stringify(eventData));
      }
      // No explicit return needed from EventsOn callback itself
    });

    // Listener for RAG context sources
    const unlistenContext = EventsOn("ragSourcesEvent", (sources: SourceInfo[]) => {
      console.log("JS: ragSourcesEvent received:", sources);
      setRagSources(sources);
    });

    if (typeof unlistenOllama === "function") {
      console.log("JS: ollamaStreamEvent listener registered successfully.");
    } else {
      console.error("JS: Failed to register ollamaStreamEvent listener. 'unlisten' is not a function:", unlistenOllama);
    }

    return () => {
      console.log("JS: App component unmounting. Cleaning up ollamaStreamEvent listener.");
      // Ensure unlisten functions are called if they exist
      if (unlistenOllama) {
        try {
          unlistenOllama();
          console.log("JS: ollamaStreamEvent listener cleaned up.");
        } catch (e) {
          console.warn("Error unsubscribing ollamaStreamEvent:", e);
        }
      }
      if (unlistenContext) {
        try {
          unlistenContext();
        } catch (e) {
          console.warn("Error unsubscribing ragContextSources:", e);
        }
      }
    };
  }, []); // Empty dependency array ensures this runs once on mount and cleans up on unmount

  const handleSendMessage = async () => {
    if (input.trim() === "" || isLoading || isDataLoading) {
      // Also disable send if data is loading
      return;
    }
    setIsLoading(true);
    setRagSources([]); // Clear previous RAG sources

    const newUserMessage: Message = {
      id: Date.now(),
      text: input,
      sender: "user",
    };

    const newAiMessageId = Date.now() + 1;
    currentAiMessageIdRef.current = newAiMessageId;
    const newAiMessagePlaceholder: Message = {
      id: newAiMessageId,
      text: "", // Initially empty, will be filled by stream. Loader will be shown here.
      sender: "ai",
    };

    // Add user message and AI placeholder to state
    setMessages((prevMessages) => [...prevMessages, newUserMessage, newAiMessagePlaceholder]);

    const currentInput = input;
    setInput("");

    try {
      await HandleMessage(currentInput);
      // If HandleMessage completes without throwing an error,
      // it means the message was sent to the Go backend successfully.
      // Streaming will be handled by the EventsOn listener.
      // The 'isLoading' state will be set to false by the 'done' event from the stream.
    } catch (error) {
      console.error("Error calling HandleMessage (JS):", error);
      setMessages((prevMessages) =>
        prevMessages.map((msg) =>
          msg.id === newAiMessageId ? { ...msg, text: `Sorry, an error occurred: ${error}`, isError: true } : msg
        )
      );
      setIsLoading(false);
      currentAiMessageIdRef.current = null;
    }
  };

  const handleLoadData = async () => {
    setIsDataLoading(true);
    setDataLoadingStatus("Requesting directory selection from user..."); // Updated status
    setRagSources([]); // Clear RAG sources when loading new data
    try {
      // Call LoadPersonalData without arguments, as it now handles the dialog internally
      const result = await LoadPersonalData();
      setDataLoadingStatus(result); // Display result from Go
      console.log("JS: LoadPersonalData result:", result);
    } catch (error: any) {
      console.error("Error loading personal data:", error);
      setDataLoadingStatus(`Error loading documents: ${error.message || String(error)}`);
    } finally {
      setIsDataLoading(false);
    }
  };

  return (
    <div id="App">
      <div className="chat-container">
        <div className="message-list">
          {messages.map((msg) => {
            // <<< START ADDED CONSOLE LOG >>>
            if (msg.sender === "ai") {
              console.log(
                `Rendering AI Message ID: ${msg.id}, Done Loading: ${!(
                  isLoading && currentAiMessageIdRef.current === msg.id
                )}, Duration: ${msg.durationMs}, Speed: ${msg.runesPerSecond}, IsError: ${msg.isError}`
              );
            }
            // <<< END ADDED CONSOLE LOG >>>
            return (
              // Each message item now includes the message div and its extras div
              <div key={msg.id} className="message-item-container">
                <div
                  className={`message ${msg.sender === "user" ? "user" : "ai"}${
                    msg.sender === "ai" ? " full-width-markdown" : ""
                  }`}
                >
                  {msg.sender === "ai" ? (
                    isLoading && currentAiMessageIdRef.current === msg.id && msg.text.length === 0 ? (
                      <div className="loader-container">
                        <div className="loader"></div>
                        <span>Thinking...</span>
                      </div>
                    ) : (
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.text}</ReactMarkdown>
                    )
                  ) : (
                    msg.text
                  )}
                </div>
                {/* Display RAG sources and metrics for AI messages */}
                {msg.sender === "ai" && (
                  <div className="ai-message-extras">
                    {/* RAG Sources Display - if msg.sources is populated */}
                    {msg.sources && msg.sources.length > 0 && (
                      <div className="rag-sources-display">
                        <strong>Sources:</strong>
                        <ul>
                          {msg.sources.map((source, index) => (
                            <li key={index}>
                              {source.fileName} (Chunk ID: {source.chunkId}, Score: {source.score.toFixed(4)})
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {/* Metrics Display - only if not currently loading this message and metrics exist */}
                    {!(isLoading && currentAiMessageIdRef.current === msg.id) &&
                      (msg.durationMs !== undefined || msg.runesPerSecond !== undefined) && (
                        <div className="ai-message-metrics">
                          {msg.durationMs !== undefined && (
                            <span>Generated in: {(msg.durationMs / 1000).toFixed(2)}s</span>
                          )}
                          {msg.durationMs !== undefined && msg.runesPerSecond !== undefined && <span> | </span>}
                          {msg.runesPerSecond !== undefined && (
                            <span>Speed: {msg.runesPerSecond.toFixed(1)} runes/s</span>
                          )}
                          {msg.isError && <span className="error-indicator"> (Error processing response)</span>}
                        </div>
                      )}
                  </div>
                )}
              </div>
            );
          })}
          <div ref={messageEndRef} />
        </div>

        {/* Display RAG Sources */}
        {ragSources && ragSources.length > 0 && (
          <div className="rag-sources-container">
            <p className="rag-sources-title">Retrieved Context:</p>
            <ul className="rag-sources-list">
              {ragSources.map((source, index) => (
                <li
                  key={index}
                  className="rag-source-item"
                  title={`File: \${source.fileName}\\nChunk ID: \${source.chunkId}\\nScore: \${source.score.toFixed(4)}`}
                >
                  {source.fileName.length > 25 ? `...\${source.fileName.slice(-22)}` : source.fileName} (Score:{" "}
                  {source.score.toFixed(2)})
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="data-loading-section">
          <button className="load-data-button" onClick={handleLoadData} disabled={isDataLoading || isLoading}>
            {isDataLoading ? (
              <div style={{ display: "flex", alignItems: "center", justifyContent: "center" }}>
                <div className="loader"></div>
                <span style={{ marginLeft: "8px" }}>Loading Data...</span>
              </div>
            ) : (
              "Load Personal Data"
            )}
          </button>
          {dataLoadingStatus && <p className="data-loading-status">{dataLoadingStatus}</p>}
        </div>
        <div className="input-area">
          <input
            type="text"
            className="chat-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && !isLoading && !isDataLoading && handleSendMessage()} // Also check isDataLoading
            placeholder={
              isLoading ? "AI is thinking..." : isDataLoading ? "Loading documents..." : "Type your message..."
            }
            disabled={isLoading || isDataLoading}
          />
          <button className="send-button" onClick={handleSendMessage} disabled={isLoading || isDataLoading}>
            {isLoading ? (
              <div style={{ display: "flex", alignItems: "center", justifyContent: "center" }}>
                <div className="loader"></div>
                <span style={{ marginLeft: "8px" }}>Sending...</span>
              </div>
            ) : (
              "Send"
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

export default App;
