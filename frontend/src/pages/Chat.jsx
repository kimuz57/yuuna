import React, { useState, useEffect, useRef } from "react";
import { GoogleLogin } from "@react-oauth/google";
const BACKEND_URL = import.meta.env.VITE_BACKEND_URL || "http://localhost:8080";
export default function Chat() {
  const [messages, setMessages] = useState(() => {
    const saved = localStorage.getItem("guestHistory");
    return saved ? JSON.parse(saved) : [];
  });

  const [messageCount, setMessageCount] = useState(() => {
    return parseInt(localStorage.getItem("messageCount")) || 0;
  });

  const [input, setInput] = useState("");
  const [showLoginPopup, setShowLoginPopup] = useState(false);
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [isTyping, setIsTyping] = useState(false);
  const [userProfile, setUserProfile] = useState(null);

  const chatEndRef = useRef(null);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isTyping]);

  useEffect(() => {
    const savedUser = localStorage.getItem("user");
    if (savedUser) {
      setIsLoggedIn(true);
      setUserProfile(JSON.parse(savedUser));
      fetchChatHistory(); // ไม่ต้องส่ง token ไปแล้ว เพราะใช้ Cookie แทน
    }
  }, []);

  const fetchChatHistory = async (token) => {
    try {
      const res = await fetch(`${BACKEND_URL}/api/history`, {
        headers: { Authorization: `Bearer ${token}` },
        credentials: "include",
      });
      if (res.ok) {
        const history = await res.json();
        if (history && history.length > 0) setMessages(history);
      }
    } catch (error) {
      console.error("Failed to load history:", error);
    }
  };

  const handleSend = async (customText, historyOverride = null) => {
    const textToSend = customText || input;
    if (!textToSend.trim() || isTyping) return;

    if (!isLoggedIn && messageCount >= 5) {
      setShowLoginPopup(true);
      return;
    }

    setInput("");
    setIsTyping(true);

    const newMessage = { role: "user", content: textToSend };
    let currentMessages = historyOverride
      ? [...historyOverride, newMessage]
      : [...messages, newMessage];

    setMessages(currentMessages);

    if (!isLoggedIn) {
      const newCount = messageCount + 1;
      setMessageCount(newCount);
      localStorage.setItem("messageCount", newCount);
      localStorage.setItem("guestHistory", JSON.stringify(currentMessages));
    }

    const endpoint = isLoggedIn
      ? `${BACKEND_URL}/api/chat`
      : `${BACKEND_URL}/api/guest/chat`;
    const headers = { "Content-Type": "application/json" };

    const payload = isLoggedIn
      ? { message: textToSend }
      : { message: textToSend, history: currentMessages.slice(0, -1) };

    try {
      const res = await fetch(endpoint, {
        method: "POST",
        headers: headers,
        body: JSON.stringify(payload),
        credentials: "include",
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        if (res.status === 403 && errData.error === "QUOTA_EXCEEDED") {
          setShowLoginPopup(true);
          setIsTyping(false);
          setMessages((prev) => prev.slice(0, -1));
          setInput(textToSend);
          return;
        }
        throw new Error(errData.error || `Server error ${res.status}`);
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder("utf-8");

      currentMessages = [...currentMessages, { role: "model", content: "" }];
      setMessages(currentMessages);

      let aiResponseText = "";

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        const chunk = decoder.decode(value, { stream: true });
        const lines = chunk.split("\n");

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            const dataStr = line.replace("data: ", "").trim();
            if (dataStr === "[DONE]") break;

            try {
              const parsed = JSON.parse(dataStr);
              if (parsed.error) {
                aiResponseText =
                  "ขออภัยค่ะ ยูนะเกิดข้อผิดพลาดในการเชื่อมต่อ (เซิร์ฟเวอร์อาจจะทำงานหนักไปหน่อย) 😭";
                setMessages((prev) => {
                  const newMsgs = [...prev];
                  newMsgs[newMsgs.length - 1] = {
                    role: "model",
                    content: aiResponseText,
                    isError: true,
                  };
                  return newMsgs;
                });
                break;
              }

              if (parsed.text) {
                aiResponseText += parsed.text;
                setMessages((prev) => {
                  const newMsgs = [...prev];
                  newMsgs[newMsgs.length - 1] = {
                    role: "model",
                    content: aiResponseText,
                  };
                  return newMsgs;
                });
              }
            } catch (e) {}
          }
        }
      }

      if (!isLoggedIn) {
        currentMessages[currentMessages.length - 1].content = aiResponseText;
        localStorage.setItem(
          "guestHistory",
          JSON.stringify([...currentMessages]),
        );
      }
    } catch (error) {
      console.error("Chat error:", error);
      setMessages((prev) => {
        const newMsgs = [...prev];
        if (
          newMsgs.length > 0 &&
          newMsgs[newMsgs.length - 1].role === "model"
        ) {
          newMsgs[newMsgs.length - 1] = {
            role: "model",
            content:
              "ขออภัยค่ะ ยูนะเชื่อมต่อไม่สำเร็จ กรุณาลองใหม่อีกครั้งนะคะ 🥺",
            isError: true,
          };
        }
        return newMsgs;
      });
    } finally {
      setIsTyping(false);
    }
  };

  const handleResend = (index) => {
    const textToResend = messages[index].content;
    const historyBefore = messages.slice(0, index);
    handleSend(textToResend, historyBefore);
  };

  const handleGoogleSuccess = async (credentialResponse) => {
    try {
      const res = await fetch(`${BACKEND_URL}/api/auth/google`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id_token: credentialResponse.credential }),
        credentials: "include",
      });

      const data = await res.json();

      // ✅ เปลี่ยนมาเช็คแค่ res.ok และ data.user พอครับ เพราะ token ไปอยู่ใน HttpOnly Cookie แล้ว
      if (res.ok && data.user) {
        localStorage.setItem("user", JSON.stringify(data.user));
        setUserProfile(data.user);
        setIsLoggedIn(true);
        setShowLoginPopup(false);

        const guestHistory = JSON.parse(
          localStorage.getItem("guestHistory") || "[]",
        );
        if (guestHistory.length > 0) {
          await fetch(`${BACKEND_URL}/api/history/migrate`, {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            credentials: "include", // 👈 แนบ Cookie ไปยืนยันตัวตนตอนย้ายประวัติแชท
            body: JSON.stringify({ history: guestHistory }),
          });

          localStorage.removeItem("guestHistory");
          localStorage.removeItem("messageCount");
        }

        fetchChatHistory(); // ดึงประวัติแชทด้วย Cookie
      }
    } catch (err) {
      console.error("Network error:", err);
    }
  };

  return (
    <div className="flex flex-col h-screen bg-[#131314] text-[#e3e3e3] antialiased selection:bg-[#2b394e]">
      {/* Top Navbar */}
      <header className="h-14 px-5 flex items-center justify-between border-b border-[#282a2c]/60 bg-[#131314]/90 backdrop-blur-md sticky top-0 z-20">
        <div className="flex items-center gap-2.5">
          <span className="text-lg font-medium tracking-tight text-[#e3e3e3]">
            Yuuna
          </span>
          {/* <span className="text-[11px] text-[#a8c7fa] bg-[#1a2333] border border-[#2b394e] px-2 py-0.5 rounded-md font-medium">
            2.5 Flash
          </span> */}
        </div>

        <div>
          {isLoggedIn ? (
            <div className="flex items-center gap-3">
              <span className="text-xs text-[#c4c7c5] hidden sm:inline">
                {userProfile?.name || "โอโตะสะมะ"}
              </span>
              <img
                src={
                  userProfile?.picture ||
                  "https://api.dicebear.com/7.x/bottts/svg?seed=Yuuna"
                }
                alt="Avatar"
                className="w-7 h-7 rounded-full border border-[#444746] object-cover"
              />
            </div>
          ) : (
            <div className="flex items-center gap-2">
              {/* <span className="text-[11px] text-[#8e918f] hidden sm:inline">
                โควตา ({messageCount}/5)
              </span> */}
              <button
                onClick={() => setShowLoginPopup(true)}
                className="px-3.5 py-1.5 bg-[#a8c7fa] hover:bg-[#8ab4f8] text-[#040e1b] text-xs font-medium rounded-full transition"
              >
                ลงชื่อเข้าใช้
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Chat Area */}
      <main className="flex-1 overflow-y-auto px-4 py-6 md:px-0">
        <div className="max-w-2xl mx-auto flex flex-col space-y-5">
          {/* Empty State */}
          {messages.length === 0 && (
            <div className="mt-12 text-left px-4">
              <h1 className="text-3xl sm:text-4xl font-semibold tracking-tight mb-2 bg-gradient-to-r from-[#4485f4] via-[#9b72cb] to-[#d96570] bg-clip-text text-transparent">
                สวัสดีค่ะ โอโตะสะมะ
              </h1>
              <p className="text-[#8e918f] text-sm mb-6">
                วันนี้มีเรื่องอะไรอยากปรึกษาหรือให้ยูนะช่วยไหมคะ?
              </p>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                {[
                  {
                    title: "แนะนำตัวหน่อยสิ",
                    desc: "ให้ยูนะเล่าเรื่องราวของเธอให้ฟัง",
                  },
                  {
                    title: "วันนี้เหนื่อยมากเลย",
                    desc: "ขอกำลังใจและคำอ้อนหวานๆ จากยูนะ",
                  },
                  {
                    title: "โลกแฟนตาซีเป็นยังไง?",
                    desc: "ถามเกี่ยวกับมอนสเตอร์และเวทมนตร์",
                  },
                  {
                    title: "แนะนำของอร่อยสำหรับมื้อเย็น",
                    desc: "คิดเมนูเด็ดๆ เอาใจโอโตะสะมะ",
                  },
                ].map((card, i) => (
                  <button
                    key={i}
                    onClick={() => handleSend(card.title)}
                    className="p-3.5 bg-[#1e1f20] hover:bg-[#282a2c] border border-[#2f3032] rounded-xl transition text-left group"
                  >
                    <div className="text-xs font-medium text-[#e3e3e3] mb-1">
                      {card.title}
                    </div>
                    <div className="text-[11px] text-[#8e918f]">
                      {card.desc}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Messages */}
          {messages.map((msg, idx) => (
            <div
              key={idx}
              className={`flex items-start gap-3 ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              {msg.role !== "user" && (
                <div
                  className={`w-7 h-7 rounded-full flex items-center justify-center text-xs shadow-sm shrink-0 mt-0.5 ${msg.isError ? "bg-[#3b1d1d] text-[#f2b8b5]" : "bg-[#1e1f20] text-[#a8c7fa]"}`}
                >
                  {msg.isError ? "⚠️" : "✦"}
                </div>
              )}

              <div className="flex flex-col max-w-[85%] sm:max-w-[80%]">
                <div
                  className={`p-3.5 rounded-2xl text-[13.5px] leading-relaxed break-words ${
                    msg.role === "user"
                      ? "bg-[#282a2c] text-[#e3e3e3] rounded-tr-sm self-end"
                      : msg.isError
                        ? "bg-[#2a1b1c] text-[#f2b8b5] border border-[#442726] rounded-tl-sm"
                        : "bg-transparent text-[#e3e3e3] pl-0"
                  }`}
                >
                  <div className="whitespace-pre-wrap">{msg.content}</div>
                </div>

                {/* Resend button */}
                {msg.role === "user" && messages[idx + 1]?.isError && (
                  <div className="mt-1.5 text-right">
                    <button
                      onClick={() => handleResend(idx)}
                      className="inline-flex items-center gap-1 px-2.5 py-1 bg-[#1e1f20] hover:bg-[#282a2c] border border-[#3c4043] text-[#a8c7fa] text-[11px] font-medium rounded-md transition"
                    >
                      <svg
                        className="w-3 h-3"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth="2"
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                        />
                      </svg>
                      ลองส่งใหม่
                    </button>
                  </div>
                )}
              </div>

              {msg.role === "user" && (
                <div className="w-7 h-7 rounded-full bg-[#282a2c] text-[#c4c7c5] flex items-center justify-center text-[11px] font-medium shrink-0 mt-0.5">
                  {userProfile?.name ? userProfile.name[0].toUpperCase() : "U"}
                </div>
              )}
            </div>
          ))}

          {/* Typing Indicator */}
          {isTyping && messages[messages.length - 1]?.role === "user" && (
            <div className="flex items-start gap-3">
              <div className="w-7 h-7 rounded-full bg-[#1e1f20] text-[#a8c7fa] flex items-center justify-center text-xs shrink-0 mt-0.5">
                ✦
              </div>
              <div className="py-2 flex items-center gap-2 text-[13px] text-[#c4c7c5]">
                <div className="flex items-center gap-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-[#a8c7fa] animate-pulse"></span>
                  <span className="w-1.5 h-1.5 rounded-full bg-[#a8c7fa] animate-pulse delay-75"></span>
                  <span className="w-1.5 h-1.5 rounded-full bg-[#a8c7fa] animate-pulse delay-150"></span>
                </div>
                <span>Yuuna กำลังคิดอยู่นะคะ ❤️</span>
              </div>
            </div>
          )}

          <div ref={chatEndRef} />
        </div>
      </main>

      {/* Gemini-style Capsule Footer */}
      <footer className="p-3 pb-4 bg-gradient-to-t from-[#131314] via-[#131314] to-transparent sticky bottom-0">
        <div className="max-w-2xl mx-auto">
          <div className="flex items-center bg-[#1e1f20] hover:bg-[#232426] focus-within:bg-[#1e1f20] border border-transparent focus-within:border-[#3c4043] rounded-full px-4 py-1.5 transition-all">
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSend()}
              disabled={isTyping}
              className="flex-1 bg-transparent py-2 outline-none text-[#e3e3e3] placeholder-[#8e918f] text-xs sm:text-sm"
              placeholder={
                isTyping ? "Yuuna กำลังคิดอยู่นะคะ ❤️" : "ส่งข้อความถึงยูนะ..."
              }
            />

            <button
              onClick={() => handleSend()}
              disabled={isTyping || !input.trim()}
              className="w-8 h-8 rounded-full bg-[#a8c7fa] text-[#040e1b] hover:bg-[#8ab4f8] flex items-center justify-center disabled:bg-transparent disabled:text-[#444746] transition ml-1"
            >
              <svg
                className="w-3.5 h-3.5 transform rotate-90"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2.5"
                  d="M12 19V5m0 0l-7 7m7-7l7 7"
                />
              </svg>
            </button>
          </div>

          <div className="text-center mt-2">
            <span className="text-[11px] text-[#8e918f]">
              Yuuna เป็น AI และอาจทำผิดพลาดได้
            </span>
          </div>
        </div>
      </footer>

      {/* Google Login Modal (Dark Mode) */}
      {showLoginPopup && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-[#1e1f20] border border-[#2f3032] rounded-2xl p-6 max-w-sm w-full relative shadow-2xl">
            <button
              onClick={() => setShowLoginPopup(false)}
              className="absolute top-4 right-4 text-[#8e918f] hover:text-[#e3e3e3] text-lg leading-none"
            >
              ✕
            </button>

            <div className="text-center">
              <div className="w-10 h-10 rounded-full bg-[#282a2c] text-[#a8c7fa] flex items-center justify-center text-lg mx-auto mb-3">
                ✦
              </div>
              <h2 className="text-base font-semibold text-[#e3e3e3] mb-1.5">
                เข้าสู่ระบบเพื่อคุยต่อ
              </h2>
              <p className="text-[#8e918f] text-xs mb-5 leading-relaxed">
                คุณได้สนทนาครบโควตาทดลองแล้ว เข้าสู่ระบบด้วย Google
                เพื่อแชทต่อและบันทึกความทรงจำกับยูนะ
              </p>

              <div className="flex justify-center my-1">
                <GoogleLogin
                  onSuccess={handleGoogleSuccess}
                  onError={() => console.log("Login Failed")}
                  //   useOneTap
                  theme="filled_black"
                  shape="pill"
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
