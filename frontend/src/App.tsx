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

function App() {
  const [input, setInput] = useState<string>("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [dataLoadStatus, setDataLoadStatus] = useState<string>("");
  const [isLoading, setIsLoading] = useState(false); // Loading state for chat messages
  const [isDataLoading, setIsDataLoading] = useState(false); // Loading state for personal data
  const [dataLoadingStatus, setDataLoadingStatus] = useState<string>(""); // Status message for data loading
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

    const unlisten = EventsOn("ollamaStreamEvent", (eventData: OllamaStreamEventPayload) => {
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
            if (lastMessage && lastMessage.sender === "ai") {
              lastMessage.text += eventData.content;
            } else {
              newMessages.push({ id: Date.now(), text: eventData.content, sender: "ai" });
            }
          }
        }
        return newMessages;
      });
    });

    if (typeof unlisten === "function") {
      console.log("JS: ollamaStreamEvent listener registered successfully.");
    } else {
      console.error("JS: Failed to register ollamaStreamEvent listener. 'unlisten' is not a function:", unlisten);
    }

    return () => {
      console.log("JS: App component unmounting. Cleaning up ollamaStreamEvent listener.");
      if (typeof unlisten === "function") {
        unlisten();
        console.log("JS: ollamaStreamEvent listener cleaned up.");
      } else {
        console.warn("JS: Cleanup - 'unlisten' was not a function during unmount.");
      }
    };
  }, []); // Empty dependency array ensures this runs once on mount and cleans up on unmount

  const handleSendMessage = async () => {
    if (input.trim() === "" || isLoading) {
      // Prevent sending if already loading
      return;
    }
    setIsLoading(true);

    const newUserMessage: Message = {
      id: Date.now(),
      text: input,
      sender: "user",
    };

    const newAiMessageId = Date.now() + 1;
    currentAiMessageIdRef.current = newAiMessageId;
    const newAiMessagePlaceholder: Message = {
      id: newAiMessageId,
      text: "", // Initially empty, will be filled by stream
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
    try {
      // Call LoadPersonalData without arguments, as it now handles the dialog internally
      const result = await LoadPersonalData();
      setDataLoadingStatus(result); // Display result from Go
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
              {/* Render text with preserved newlines */}
              <p style={{ whiteSpace: "pre-wrap" }}>{msg.text}</p>
            </div>
          ))}
          {isLoading &&
            currentAiMessageIdRef.current === null && ( // Show general loader if AI message isn't created yet
              <div className="message ai">
                <p>
                  <i>Preparing response...</i>
                </p>
              </div>
            )}
          <div ref={messageEndRef} />
        </div>
        <div className="data-loading-section">
          <button className="load-data-button" onClick={handleLoadData} disabled={isDataLoading}>
            {isDataLoading ? "Loading Data..." : "Load Personal Data"}
          </button>
          {dataLoadingStatus && <p className="data-loading-status">{dataLoadingStatus}</p>}
        </div>
        <div className="input-area">
          <input
            type="text"
            className="chat-input"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && !isLoading && handleSendMessage()}
            placeholder="Type your message..."
            disabled={isLoading || isDataLoading}
          />
          <button className="send-button" onClick={handleSendMessage} disabled={isLoading || isDataLoading}>
            Send
          </button>
        </div>
      </div>
    </div>
  );
}

export default App;
