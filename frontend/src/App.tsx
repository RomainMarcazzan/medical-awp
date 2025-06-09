import { useEffect, useRef, useState } from "react";
import "./App.css";
import { HandleMessage, LoadPersonalData } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime"; // Corrected import path for EventsOn

interface Message {
  id: number;
  text: string;
  sender: "user" | "ai";
}

// Define the structure of the event payload from Go
interface OllamaStreamEventPayload {
  content?: string; // content can be empty, especially in the final 'done' message
  done: boolean;
  error?: string;
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

    // Updated interface to match observed event data
    interface OllamaStreamEventData {
      content?: string; // Content is present for chunks
      done: boolean; // True when the stream is complete
    }

    const unlistenOllama = EventsOn("ollamaStreamEvent", (eventData: OllamaStreamEventPayload) => {
      // Changed to use OllamaStreamEventPayload
      console.log("JS: ollamaStreamEvent received:", eventData);

      if (eventData.error) {
        console.error("JS: Ollama stream error:", eventData.error);
        setMessages((prevMessages) => {
          const newMessages = [...prevMessages];
          const aiMessageIndex = newMessages.findIndex((msg) => msg.id === currentAiMessageIdRef.current);
          if (aiMessageIndex !== -1) {
            newMessages[aiMessageIndex] = {
              ...newMessages[aiMessageIndex],
              text:
                newMessages[aiMessageIndex].text.length > 0
                  ? `\${newMessages[aiMessageIndex].text}\n[Error: \${eventData.error}]`
                  : `[Error: \${eventData.error}]`,
            };
          } else {
            // If no placeholder, add a new error message
            newMessages.push({ id: Date.now(), text: `[Error: \${eventData.error}]`, sender: "ai" });
          }
          return newMessages;
        });
        setIsLoading(false); // Stop loading on error
        currentAiMessageIdRef.current = null; // Reset ref on error
        return; // Stop further processing for this event
      }

      setMessages((prevMessages) => {
        const newMessages = [...prevMessages];
        const aiMessageIndex = newMessages.findIndex((msg) => msg.id === currentAiMessageIdRef.current);

        if (eventData.done) {
          console.log("JS: Ollama stream processing marked as done.");
          setIsLoading(false); // Stop loading when stream is done
          currentAiMessageIdRef.current = null; // Reset ref when stream is done
          // No text update needed here, just finalize loading state
          return newMessages;
        }

        if (typeof eventData.content === "string") {
          if (aiMessageIndex !== -1) {
            newMessages[aiMessageIndex] = {
              ...newMessages[aiMessageIndex],
              text: newMessages[aiMessageIndex].text + eventData.content,
            };
          } else {
            // This case should ideally not be hit if placeholder is always added
            console.warn(
              "JS: Received stream content, but AI message placeholder not found by ID. Appending to last AI message or creating new."
            );
            const lastMessage = newMessages[newMessages.length - 1];
            if (lastMessage && lastMessage.sender === "ai" && !currentAiMessageIdRef.current) {
              // Append if no active stream ID
              lastMessage.text += eventData.content;
            } else if (!currentAiMessageIdRef.current) {
              // Create new if no active stream ID and last wasn't AI
              newMessages.push({ id: Date.now(), text: eventData.content, sender: "ai" });
            }
          }
        }
        return newMessages;
      });
    });

    // Listener for RAG context sources
    const unlistenContext = EventsOn("ragContextSources", (sources: SourceInfo[]) => {
      console.log("JS: ragContextSources received:", sources);
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
    // Use a functional update to ensure we're working with the latest state
    setMessages((prevMessages) => [...prevMessages, newUserMessage, newAiMessagePlaceholder]);

    const currentInput = input;
    setInput("");

    try {
      const initialResponse = await HandleMessage(currentInput);
      if (initialResponse) {
        // Non-empty string indicates an error from Go before streaming
        console.error("Error from HandleMessage (Go):", initialResponse);
        setMessages((prevMessages) =>
          prevMessages.map((msg) => (msg.id === newAiMessageId ? { ...msg, text: initialResponse } : msg))
        );
        setIsLoading(false);
        currentAiMessageIdRef.current = null;
      }
      // If initialResponse is empty, streaming has started or will start.
      // The EventsOn listener handles further updates and setting isLoading to false.
    } catch (error) {
      console.error("Error calling HandleMessage (JS):", error);
      setMessages((prevMessages) =>
        prevMessages.map((msg) =>
          msg.id === newAiMessageId ? { ...msg, text: "Sorry, an error occurred while sending your message." } : msg
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
          {messages.map((msg) => (
            <div key={msg.id} className={`message ${msg.sender}`}>
              {msg.sender === "ai" && isLoading && msg.text === "" && currentAiMessageIdRef.current === msg.id ? (
                <div style={{ display: "flex", alignItems: "center" }}>
                  <div className="loader"></div>
                  <span>Thinking...</span>
                </div>
              ) : (
                <p style={{ whiteSpace: "pre-wrap" }}>{msg.text}</p>
              )}
            </div>
          ))}
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
