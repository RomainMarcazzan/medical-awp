import { useEffect, useRef, useState } from "react";
import "./App.css";
import { HandleMessage } from "../wailsjs/go/main/App";

interface Message {
  id: number;
  text: string;
  sender: "user" | "ai";
}

function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [inputText, setInputText] = useState("");
  const messageEndRef = useRef<null | HTMLDivElement>(null);

  const scrollToBottom = () => {
    messageEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(scrollToBottom, [messages]);

  const handleSendMessage = async () => {
    if (inputText.trim() === "") {
      return;
    }
    const newUserMessage: Message = {
      id: Date.now(),
      text: inputText,
      sender: "user",
    };

    setMessages((prevMessages) => [...prevMessages, newUserMessage]);
    setInputText("");

    try {
      const aiResponseText = await HandleMessage(inputText);
      const newAiMessage: Message = {
        id: Date.now() + 1,
        text: aiResponseText,
        sender: "ai",
      };
      setMessages((prevMessages) => [...prevMessages, newAiMessage]);
    } catch (error) {
      console.error("Error calling HandleMessage", error);
      const errorAiMessage: Message = {
        id: Date.now() + 1,
        text: "Sorry I couldn't process your message",
        sender: "ai",
      };
      setMessages((prevMessages) => [...prevMessages, errorAiMessage]);
    }
  };

  return (
    <div id="App">
      <div className="chat-container">
        <div className="message-list">
          {messages.map((msg) => (
            <div key={msg.id} className={`message ${msg.sender}`}>
              <p>{msg.text}</p>
            </div>
          ))}
          <div ref={messageEndRef} />
        </div>
        <div className="input-area">
          <input
            type="text"
            className="chat-input"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSendMessage()}
            placeholder="Type your message..."
          />
          <button className="send-button" onClick={handleSendMessage}>
            Send
          </button>
        </div>
      </div>
    </div>
  );
}

export default App;
