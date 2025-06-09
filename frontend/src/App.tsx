import { useEffect, useRef, useState } from "react";
import "./App.css";
import { HandleMessage, LoadPersonalData } from "../wailsjs/go/main/App"; // Import LoadPersonalData
import { EventsOn, OpenDirectoryDialog } from "../wailsjs/runtime"; // Import EventsOn and OpenDirectoryDialog

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
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState("");
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
    const unsubscribe = EventsOn("ollamaStreamEvent", (data: OllamaStreamEventPayload) => {
      // Ensure data is not undefined and has the expected structure
      if (typeof data !== "object" || data === null) {
        console.error("Received malformed stream event data:", data);
        if (currentAiMessageIdRef.current !== null) {
          setMessages((prevMessages) =>
            prevMessages.map((msg) =>
              msg.id === currentAiMessageIdRef.current
                ? { ...msg, text: (msg.text || "") + "\\\\n[Error: Malformed stream event]" } // Ensure msg.text is not null
                : msg
            )
          );
        }
        setIsLoading(false);
        currentAiMessageIdRef.current = null;
        return;
      }

      if (data.error) {
        console.error("Streaming Error from Go:", data.error);
        if (currentAiMessageIdRef.current !== null) {
          setMessages((prevMessages) =>
            prevMessages.map((msg) =>
              msg.id === currentAiMessageIdRef.current
                ? { ...msg, text: (msg.text || "") + `\\\\n[Error: ${data.error}]` } // Ensure msg.text is not null
                : msg
            )
          );
        }
        setIsLoading(false);
        currentAiMessageIdRef.current = null;
        return;
      }

      if (currentAiMessageIdRef.current !== null && typeof data.content === "string") {
        setMessages((prevMessages) =>
          prevMessages.map((msg) =>
            msg.id === currentAiMessageIdRef.current
              ? { ...msg, text: (msg.text || "") + data.content } // Append content
              : msg
          )
        );
      }

      if (data.done) {
        setIsLoading(false);
        currentAiMessageIdRef.current = null;
        console.log("Streaming finished.");
      }
    });

    // Cleanup listener on component unmount
    return () => {
      // Wails V2 EventsOn returns a function to unsubscribe
      unsubscribe();
    };
  }, []); // Empty dependency array means this runs once on mount and cleans up on unmount

  const handleSendMessage = async () => {
    if (inputText.trim() === "" || isLoading) {
      // Prevent sending if already loading
      return;
    }
    setIsLoading(true);

    const newUserMessage: Message = {
      id: Date.now(),
      text: inputText,
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

    const currentInput = inputText;
    setInputText("");

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
    setDataLoadingStatus("Opening directory selection dialog...");
    try {
      const directoryPath = await OpenDirectoryDialog({
        title: "Select Folder Containing Your Documents",
      });

      if (directoryPath) {
        setDataLoadingStatus(`Loading documents from: ${directoryPath}...`);
        const result = await LoadPersonalData(directoryPath);
        setDataLoadingStatus(result); // Display result from Go (e.g., "Successfully loaded X chunks.")
      } else {
        setDataLoadingStatus("Document loading cancelled by user.");
      }
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
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
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
