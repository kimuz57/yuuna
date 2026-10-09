import React, { useState, useEffect, useRef } from "react";
import { GoogleLogin } from "@react-oauth/google";

export default function Chat() {
  // ================= STATE =================
  const [messages, setMessages] = useState(() => {
    const saved = localStorage.getItem("guestHistory");
    return saved ? JSON.parse(saved) : [];
  });

  const [messageCount, setMessageCount] = useState(() => {
    try {
      const encryptedCount = localStorage.getItem("messageCount");
      if (!encryptedCount) return 0;
      const decoded = atob(encryptedCount);
      const parts = decoded.split("yuuna_cnt_");
      return parts.length > 1 ? parseInt(parts[1], 10) || 0 : 0;
    } catch (e) {
      return 0;
    }
  });

  const [input, setInput] = useState("");
  const [showLoginPopup, setShowLoginPopup] = useState(false);
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [isTyping, setIsTyping] = useState(false);
  const [userProfile, setUserProfile] = useState(null);

  const [isInitializing, setIsInitializing] = useState(true);
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [sessions, setSessions] = useState([]);
  const [activeSessionId, setActiveSessionId] = useState(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  const chatEndRef = useRef(null);
  const [editingSessionId, setEditingSessionId] = useState(null);
  const [newSessionTitle, setNewSessionTitle] = useState("");
  const [activeMenuSessionId, setActiveMenuSessionId] = useState(null);

  // ================= HELPERS =================
  const encodeSession = (id) => {
    try {
      return btoa(`yuuna_sess_${id}`);
    } catch (e) {
      return id;
    }
  };

  const decodeSession = (encodedStr) => {
    try {
      const decoded = atob(encodedStr);
      const parts = decoded.split("yuuna_sess_");
      return parts.length > 1 ? parts[1] : null;
    } catch (e) {
      return null;
    }
  };

  const saveEncryptedMessageCount = (count) => {
    try {
      const encoded = btoa(`yuuna_cnt_${count}`);
      localStorage.setItem("messageCount", encoded);
    } catch (e) {}
  };

  // ================= EFFECTS =================
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, isTyping]);

  // เคลียร์สถานะล็อกอินเมื่อเซิร์ฟเวอร์ปฏิเสธ (401) — ไม่เชื่อ localStorage
  const handleSessionExpired = () => {
    setIsLoggedIn(false);
    setUserProfile(null);
    localStorage.removeItem("user"); // เคลียร์ค่าที่ค้างจากเวอร์ชันเก่า (ถ้ามี)
    setSessions([]);
    setActiveSessionId(null);
  };

  useEffect(() => {
    const initializeAuthAndData = async () => {
      try {
        // ตรวจสอบสถานะล็อกอินจริงกับเซิร์ฟเวอร์ทุกครั้งที่เปิดหน้าเว็บ
        // ตัดสินใจจาก HttpOnly Cookie ฝั่ง Backend เท่านั้น ไม่เชื่อ localStorage
        const res = await fetch(`/api/auth/me`, {
          credentials: "include",
        });

        if (res.ok) {
          const data = await res.json();
          setIsLoggedIn(true);
          setUserProfile(data.user);
          await fetchSessionsAndRestore();
        } else {
          handleSessionExpired();
        }
      } catch (error) {
        console.error("Session check failed:", error);
        handleSessionExpired();
      } finally {
        setIsInitializing(false);
      }
    };

    initializeAuthAndData();
  }, []);

  const fetchChatHistory = async (sessionId) => {
    if (!sessionId) return;
    try {
      const res = await fetch(
        `/api/history?session_id=${sessionId}`,
        {
          credentials: "include",
        },
      );
      if (res.status === 401) {
        handleSessionExpired();
        return;
      }
      if (res.ok) {
        const history = await res.json();
        setMessages(history || []);
        setActiveSessionId(sessionId);

        localStorage.setItem(
          "last_active_session_id",
          encodeSession(sessionId),
        );

        setTimeout(() => {
          chatEndRef.current?.scrollIntoView({ behavior: "instant" });
        }, 50);
      }
    } catch (error) {
      console.error("Failed to load history:", error);
    }
  };

  const fetchSessionsAndRestore = async () => {
    try {
      const res = await fetch(`/api/sessions`, {
        credentials: "include",
      });
      if (res.status === 401) {
        handleSessionExpired();
        return;
      }
      if (res.ok) {
        const data = await res.json();
        const sessionList = data || [];
        setSessions(sessionList);

        if (sessionList.length > 0) {
          const encryptedLastSession = localStorage.getItem(
            "last_active_session_id",
          );
          const savedLastSessionId = encryptedLastSession
            ? decodeSession(encryptedLastSession)
            : null;

          const targetSession = sessionList.find(
            (s) => s.id.toString() === savedLastSessionId,
          );

          if (targetSession) {
            await fetchChatHistory(targetSession.id);
          } else {
            localStorage.removeItem("last_active_session_id");
            setActiveSessionId(null);
            setMessages([]);
          }
        }
      }
    } catch (error) {
      console.error("Failed to load sessions:", error);
    }
  };

  const handleSelectSession = (sessionId) => {
    if (isTyping) return;
    fetchChatHistory(sessionId);
    setIsSidebarOpen(false);
  };

  const handleNewChat = () => {
    if (isTyping) return;
    setActiveSessionId(null);
    setMessages([]);
    setInput("");
    localStorage.removeItem("last_active_session_id");
    setIsSidebarOpen(false);
  };

  const handleRenameSession = async (sessionId, e) => {
    e.stopPropagation();
    if (!newSessionTitle.trim()) return;
    try {
      const res = await fetch(`/api/sessions/${sessionId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: newSessionTitle }),
        credentials: "include",
      });
      if (res.ok) {
        setSessions(
          sessions.map((s) =>
            s.id === sessionId ? { ...s, title: newSessionTitle } : s,
          ),
        );
        setEditingSessionId(null);
      }
    } catch (error) {
      console.error("Failed to rename session:", error);
    }
  };

  const handleDeleteSession = async (sessionId, e) => {
    e.stopPropagation();
    if (!window.confirm("คุณต้องการลบประวัติการสนทนานี้ใช่หรือไม่?")) return;
    try {
      const res = await fetch(`/api/sessions/${sessionId}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (res.ok) {
        setSessions(sessions.filter((s) => s.id !== sessionId));
        if (activeSessionId === sessionId) {
          handleNewChat();
        }
      }
    } catch (error) {
      console.error("Failed to delete session:", error);
    }
  };

  const handleLogout = async () => {
    try {
      await fetch(`/api/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch (err) {
      console.error("Logout error:", err);
    } finally {
      // ให้เซิร์ฟเวอร์เป็นฝั่งทำลาย HttpOnly Cookie (POST /api/auth/logout)
      // และล้างสถานะฝั่ง UI เท่านั้น — ไม่มีการตัดสินใจจาก localStorage
      localStorage.removeItem("user");
      localStorage.removeItem("last_active_session_id");
      setIsLoggedIn(false);
      setUserProfile(null);
      setMessages([]);
      setSessions([]);
      setActiveSessionId(null);
      setIsProfileOpen(false);
      setIsSidebarOpen(false);
      window.location.reload();
    }
  };

  // ================= SEND MESSAGE =================
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
      saveEncryptedMessageCount(newCount);
      localStorage.setItem("guestHistory", JSON.stringify(currentMessages));
    }

    const endpoint = isLoggedIn
      ? `/api/chat`
      : `/api/guest/chat`;

    const payload = isLoggedIn
      ? { message: textToSend, session_id: activeSessionId }
      : { message: textToSend, history: currentMessages.slice(0, -1) };

    try {
      const res = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
        credentials: "include",
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        if (res.status === 401) {
          // Session หมดอายุระหว่างใช้งาน — เคลียร์สถานะแล้วกลับสู่สถานะผู้เยี่ยมชม
          handleSessionExpired();
          setShowLoginPopup(true);
          setMessages((prev) => prev.slice(0, -1));
          setInput(textToSend);
          return;
        }
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
        if (done) {
          if (isLoggedIn && !activeSessionId) {
            const sessRes = await fetch(`/api/sessions`, {
              credentials: "include",
            });
            if (sessRes.ok) {
              const sessData = await sessRes.json();
              if (sessData && sessData.length > 0) {
                const latestSession = sessData[0];
                setActiveSessionId(latestSession.id);
                localStorage.setItem(
                  "last_active_session_id",
                  encodeSession(latestSession.id),
                );
                setSessions(sessData);
              }
            }
          }
          break;
        }

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
                  "ขออภัยค่ะ ยูนะเกิดข้อผิดพลาดในการเชื่อมต่อ 😭";
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
      const res = await fetch(`/api/auth/google`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id_token: credentialResponse.credential }),
        credentials: "include",
      });

      const data = await res.json();

      if (res.ok && data.user) {
        // รอผลยืนยันจาก Backend ก่อนอัปเดตสถานะ (เซิร์ฟเวอร์ออก HttpOnly Cookie แล้ว)
        // ไม่เขียน user ลง localStorage — สถานะล็อกอินถูกเช็กผ่าน GET /api/auth/me เท่านั้น
        setUserProfile(data.user);
        setIsLoggedIn(true);
        setShowLoginPopup(false);

        localStorage.removeItem("guestHistory");
        localStorage.removeItem("messageCount");
        setMessageCount(0);

        fetchSessionsAndRestore();
      } else {
        console.error("Login failed:", data.error || "Unknown error");
      }
    } catch (err) {
      console.error("Network error:", err);
    }
  };

  if (isInitializing) {
    return (
      <div className="flex h-screen w-screen bg-[#131314] items-center justify-center text-[#8e918f] text-sm">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-[#a8c7fa] animate-pulse"></span>
          <span>กำลังโหลดความทรงจำกับยูนะ...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-[#131314] text-[#e3e3e3] antialiased selection:bg-[#2b394e] relative overflow-hidden">
      {/* ================= SIDEBAR ================= */}
      {isLoggedIn && (
        <>
          {isSidebarOpen && (
            <div
              className="fixed inset-0 bg-black/60 z-30 md:hidden backdrop-blur-xs"
              onClick={() => setIsSidebarOpen(false)}
            />
          )}

          <aside
            className={`
            fixed md:static inset-y-0 left-0 z-40
            w-[280px] bg-[#1e1f20] border-r border-[#282a2c]/60 
            flex flex-col shrink-0 h-screen transition-transform duration-300 ease-in-out
            ${isSidebarOpen ? "translate-x-0" : "-translate-x-full md:translate-x-0"}
          `}
          >
            <div className="p-4 border-b border-[#282a2c]/40 flex items-center justify-between">
              <button
                type="button"
                onClick={handleNewChat}
                className={`w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl text-sm sm:text-base font-medium transition cursor-pointer shadow-xs ${
                  activeSessionId === null
                    ? "bg-[#444746] text-[#e3e3e3] border border-[#8e918f]"
                    : "bg-[#282a2c] hover:bg-[#3c4043] text-[#e3e3e3]"
                }`}
              >
                <svg
                  className="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2.5"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
                แชทใหม่
              </button>

              <button
                onClick={() => setIsSidebarOpen(false)}
                className="md:hidden ml-2 p-2 text-[#8e918f] hover:text-white"
              >
                ✕
              </button>
            </div>

            <div className="flex-1 overflow-y-auto px-3 py-3 space-y-1">
              <div className="text-xs font-semibold text-[#8e918f] uppercase tracking-wider px-2 mb-3">
                ประวัติการสนทนา
              </div>

              {sessions.map((item) => (
                <div
                  key={item.id}
                  onClick={() => {
                    handleSelectSession(item.id);
                    setActiveMenuSessionId(null); // ปิดเมนูไข่ปลาถ้ากดเลือกแชท
                  }}
                  className={`w-full group relative text-left px-3 py-2.5 rounded-xl text-sm sm:text-[15px] transition cursor-pointer flex items-center justify-between ${
                    activeSessionId === item.id
                      ? "bg-[#282a2c] text-white font-medium shadow-xs"
                      : "text-[#c4c7c5] hover:bg-[#282a2c]/50 hover:text-white"
                  }`}
                >
                  <div className="flex items-center gap-3 truncate flex-1 mr-2">
                    <svg
                      className="w-4 h-4 shrink-0 opacity-60"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="2"
                        d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
                      />
                    </svg>

                    {editingSessionId === item.id ? (
                      <input
                        type="text"
                        value={newSessionTitle}
                        onChange={(e) => setNewSessionTitle(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === "Enter")
                            handleRenameSession(item.id, e);
                          if (e.key === "Escape") setEditingSessionId(null);
                        }}
                        onClick={(e) => e.stopPropagation()}
                        autoFocus
                        className="bg-[#131314] text-white px-2 py-0.5 rounded text-sm w-full outline-none border border-[#444746]"
                      />
                    ) : (
                      <span className="truncate">
                        {item.title || "แชทกับยูนะ"}
                      </span>
                    )}
                  </div>

                  {/* ฝั่งขวา: สำหรับคอม (Hover) และสำหรับมือถือ (ปุ่มไข่ปลา) */}
                  <div className="flex items-center gap-1 shrink-0">
                    {editingSessionId !== item.id ? (
                      <>
                        {/* 💻 ปุ่มสำหรับหน้าจอคอม (แสดงเมื่อ Hover) */}
                        <div className="hidden md:flex opacity-0 group-hover:opacity-100 transition-opacity items-center gap-1">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              setEditingSessionId(item.id);
                              setNewSessionTitle(item.title || "");
                            }}
                            className="p-1.5 text-[#8e918f] hover:text-white hover:bg-[#3c4043]/50 rounded-lg transition"
                            title="เปลี่ยนชื่อแชท"
                          >
                            <svg
                              className="w-4 h-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth="2"
                                d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                              />
                            </svg>
                          </button>
                          <button
                            type="button"
                            onClick={(e) => handleDeleteSession(item.id, e)}
                            className="p-1.5 text-[#8e918f] hover:text-[#f2b8b5] hover:bg-[#3b1d1d]/50 rounded-lg transition"
                            title="ลบแชท"
                          >
                            <svg
                              className="w-4 h-4"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth="2"
                                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                              />
                            </svg>
                          </button>
                        </div>

                        {/* 📱 ปุ่มไข่ปลา (Three-dot Menu) สำหรับมือถือ */}
                        <div className="relative md:hidden -mr-3">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              setActiveMenuSessionId(
                                activeMenuSessionId === item.id
                                  ? null
                                  : item.id,
                              );
                            }}
                            className="p-1.5 text-[#8e918f] hover:text-white rounded-lg transition "
                            title="ตัวเลือกเพิ่มเติม"
                          >
                            <svg
                              className="w-5 h-5"
                              fill="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
                            </svg>
                          </button>

                          {/* Popup เมนูเมื่อกดปุ่มไข่ปลาบนมือถือ (ใช้ SVG แทนอีโมจิทั้งหมด) */}
                          {activeMenuSessionId === item.id && (
                            <div className="absolute right-0 top-8 w-36 bg-[#282a2c] border border-[#444746] rounded-xl shadow-xl py-1.5 z-30">
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setEditingSessionId(item.id);
                                  setNewSessionTitle(item.title || "");
                                  setActiveMenuSessionId(null);
                                }}
                                className="w-full text-left px-3 py-2 text-xs text-[#e3e3e3] hover:bg-[#3c4043] flex items-center gap-2.5 transition"
                              >
                                <svg
                                  className="w-4 h-4 text-[#8e918f]"
                                  fill="none"
                                  stroke="currentColor"
                                  viewBox="0 0 24 24"
                                >
                                  <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth="2"
                                    d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                                  />
                                </svg>
                                แก้ไขชื่อ
                              </button>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setActiveMenuSessionId(null);
                                  handleDeleteSession(item.id, e);
                                }}
                                className="w-full text-left px-3 py-2 text-xs text-[#f2b8b5] hover:bg-[#3b1d1d] flex items-center gap-2.5 transition"
                              >
                                <svg
                                  className="w-4 h-4 text-[#f2b8b5]"
                                  fill="none"
                                  stroke="currentColor"
                                  viewBox="0 0 24 24"
                                >
                                  <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth="2"
                                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                                  />
                                </svg>
                                ลบแชท
                              </button>
                            </div>
                          )}
                        </div>
                      </>
                    ) : (
                      <button
                        type="button"
                        onClick={(e) => handleRenameSession(item.id, e)}
                        className="text-xs text-[#a8c7fa] px-2 py-1 bg-[#3c4043] hover:bg-[#444746] rounded-md transition"
                      >
                        บันทึก
                      </button>
                    )}
                  </div>
                </div>
              ))}

              {sessions.length === 0 && (
                <p className="text-sm text-[#8e918f] text-center mt-6">
                  ยังไม่มีประวัติแชท
                </p>
              )}
            </div>
          </aside>
        </>
      )}

      {/* ================= MAIN CONTENT ================= */}
      <div className="flex-1 flex flex-col relative w-full overflow-hidden">
        {/* Top Navbar */}
        <header className="h-16 px-4 md:px-6 flex items-center justify-between border-b border-[#282a2c]/60 bg-[#131314]/90 backdrop-blur-md sticky top-0 z-20">
          <div className="flex items-center">
            {isLoggedIn && (
              <button
                onClick={() => setIsSidebarOpen(true)}
                className="-mr-3 p-2 -ml-2 text-[#c4c7c5] hover:text-white md:hidden focus:outline-none cursor-pointer"
                title="เปิดเมนูประวัติแชท"
              >
                <svg
                  className="w-6 h-6"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M4 6h16M4 12h16M4 18h16"
                  />
                </svg>
              </button>
            )}

            {/* 🌟 Flat Faceted Y Logo */}
            <div className="flex items-center -ml-1">
              <img
                src="logo.png"
                alt="Yuuna Logo"
                className="w-13 h-13 object-contain -mr-4 -mt-3"
              />
              <span className="text-xl font-medium tracking-tight text-[#e3e3e3]">
                uuna
              </span>
            </div>
          </div>

          <div>
            {isLoggedIn ? (
              <div className="relative">
                <button
                  onClick={() => setIsProfileOpen(!isProfileOpen)}
                  className="flex items-center gap-3 p-1.5 rounded-full hover:bg-[#282a2c] transition focus:outline-none cursor-pointer"
                >
                  <span className="text-sm text-[#c4c7c5] hidden sm:inline">
                    {userProfile?.name || "โอโตะสะมะ"}
                  </span>
                  <img
                    src={
                      userProfile?.picture ||
                      "https://api.dicebear.com/7.x/bottts/svg?seed=Yuuna"
                    }
                    alt="Avatar"
                    className="w-9 h-9 rounded-full border border-[#444746] object-cover"
                  />
                </button>

                {isProfileOpen && (
                  <>
                    <div
                      className="fixed inset-0 z-30"
                      onClick={() => setIsProfileOpen(false)}
                    ></div>
                    <div className="absolute right-0 mt-3 w-64 bg-[#1e1f20] border border-[#2f3032] rounded-2xl shadow-xl z-40 overflow-hidden">
                      <div className="p-4 border-b border-[#2f3032] flex items-center gap-4">
                        <img
                          src={
                            userProfile?.picture ||
                            "https://api.dicebear.com/7.x/bottts/svg?seed=Yuuna"
                          }
                          alt="Avatar"
                          className="w-12 h-12 rounded-full object-cover shrink-0"
                        />
                        <div className="overflow-hidden">
                          <p className="text-base font-medium text-white truncate">
                            {userProfile?.name}
                          </p>
                          <p className="text-sm text-[#8e918f] truncate">
                            {userProfile?.email}
                          </p>
                        </div>
                      </div>
                      <div className="p-2">
                        <button
                          onClick={handleLogout}
                          className="w-full text-left px-4 py-2.5 text-[15px] text-[#f2b8b5] hover:bg-[#3b1d1d] hover:text-[#f2b8b5] rounded-xl transition cursor-pointer"
                        >
                          ออกจากระบบ
                        </button>
                      </div>
                    </div>
                  </>
                )}
              </div>
            ) : (
              <button
                onClick={() => setShowLoginPopup(true)}
                className="px-4 py-2 bg-[#a8c7fa] hover:bg-[#8ab4f8] text-[#040e1b] text-sm font-medium rounded-full transition cursor-pointer"
              >
                ลงชื่อเข้าใช้
              </button>
            )}
          </div>
        </header>

        {/* Chat Area */}
        <main className="flex-1 overflow-y-auto px-3 py-8 sm:px-6 md:px-8 scroll-smooth">
          <div className="max-w-5xl mx-auto px-1 sm:px-4 flex flex-col space-y-6">
            {messages.length === 0 && (
              <div className="mt-12 text-left px-4">
                <h1 className="text-4xl sm:text-5xl font-semibold tracking-tight mb-3 bg-gradient-to-r from-[#4485f4] via-[#9b72cb] to-[#d96570] bg-clip-text text-transparent">
                  สวัสดีค่ะ{" "}
                  {userProfile ? userProfile.name.split(" ")[0] : "โอโตะสะมะ"}
                </h1>
                <p className="text-[#8e918f] text-base mb-8">
                  วันนี้มีเรื่องอะไรอยากปรึกษาหรือให้ยูนะช่วยไหมคะ?
                </p>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
                      className="p-4 bg-[#1e1f20] hover:bg-[#282a2c] border border-[#2f3032] rounded-xl transition text-left group cursor-pointer"
                    >
                      <div className="text-sm font-medium text-[#e3e3e3] mb-1.5">
                        {card.title}
                      </div>
                      <div className="text-xs text-[#8e918f]">{card.desc}</div>
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Messages */}
            {messages.map((msg, idx) => (
              <div
                key={idx}
                className={`flex items-start gap-3 sm:gap-4 ${msg.role === "user" ? "justify-end" : "justify-start"}`}
              >
                {msg.role !== "user" && (
                  <div
                    className={`mt-3 w-7 h-7 sm:w-8 sm:h-8 rounded-full flex items-center justify-center text-xs sm:text-sm shadow-sm shrink-0 mt-0.5 ml-1 sm:ml-2 ${msg.isError ? "bg-[#3b1d1d] text-[#f2b8b5]" : "bg-[#1e1f20] text-[#a8c7fa]"}`}
                  >
                    {msg.isError ? (
                      "⚠️"
                    ) : (
                      <img
                        src="logo.png"
                        alt="Yuuna Logo"
                        className="w-full h-full object-contain"
                      />
                    )}
                  </div>
                )}

                <div className="flex flex-col max-w-[85%] sm:max-w-[80%]">
                  <div
                    className={`p-4 rounded-2xl text-[15px] leading-relaxed break-words ${
                      msg.role === "user"
                        ? "bg-[#282a2c] text-[#e3e3e3] rounded-tr-sm self-end"
                        : msg.isError
                          ? "bg-[#2a1b1c] text-[#f2b8b5] border border-[#442726] rounded-tl-sm"
                          : "bg-transparent text-[#e3e3e3] pl-0"
                    }`}
                  >
                    <div className="whitespace-pre-wrap">{msg.content}</div>
                  </div>

                  {msg.role === "user" && messages[idx + 1]?.isError && (
                    <div className="mt-2 text-right">
                      <button
                        onClick={() => handleResend(idx)}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-[#1e1f20] hover:bg-[#282a2c] border border-[#3c4043] text-[#a8c7fa] text-xs font-medium rounded-md transition cursor-pointer"
                      >
                        <svg
                          className="w-3.5 h-3.5"
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
                  <div className="w-7 h-7 sm:w-8 sm:h-8 rounded-full bg-[#282a2c] text-[#c4c7c5] flex items-center justify-center text-xs font-medium shrink-0 mt-0.5 overflow-hidden">
                    {userProfile?.picture ? (
                      <img
                        src={userProfile.picture}
                        alt="U"
                        className="w-full h-full object-cover"
                      />
                    ) : userProfile?.name ? (
                      userProfile.name[0].toUpperCase()
                    ) : (
                      "U"
                    )}
                  </div>
                )}
              </div>
            ))}

            {isTyping && messages[messages.length - 1]?.role === "user" && (
              <div className="flex items-start gap-4">
                <div className="w-8 h-8 rounded-full bg-[#1e1f20] text-[#a8c7fa] flex items-center justify-center text-sm shrink-0 mt-0.5">
                  <img
                    src="logo.png"
                    alt="Yuuna Logo"
                    className="w-full h-full object-contain"
                  />
                </div>
                <div className="py-2.5 flex items-center gap-2.5 text-[14.5px] text-[#c4c7c5]">
                  <div className="flex items-center gap-1.5">
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

        {/* Input Footer */}
        <footer className="p-4 pb-6 px-4 sm:px-6 bg-gradient-to-t from-[#131314] via-[#131314] to-transparent sticky bottom-0">
          <div className="max-w-5xl mx-auto px-1 sm:px-2">
            <div className="flex items-center bg-[#1e1f20] hover:bg-[#232426] focus-within:bg-[#1e1f20] border border-transparent focus-within:border-[#3c4043] rounded-full px-5 py-2 transition-all">
              <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleSend()}
                disabled={isTyping}
                className="flex-1 bg-transparent py-2.5 outline-none text-[#e3e3e3] placeholder-[#8e918f] text-sm sm:text-[15px]"
                placeholder={
                  isTyping
                    ? "Yuuna กำลังคิดอยู่นะคะ ❤️"
                    : "ส่งข้อความถึงยูนะ..."
                }
              />
              <button
                onClick={() => handleSend()}
                disabled={isTyping || !input.trim()}
                className="w-10 h-10 rounded-full bg-[#a8c7fa] text-[#040e1b] hover:bg-[#8ab4f8] flex items-center justify-center disabled:bg-transparent disabled:text-[#444746] transition ml-2 cursor-pointer"
              >
                <svg
                  className="w-4 h-4 transform rotate-90"
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
          </div>
        </footer>
      </div>

      {/* Google Login Popup */}
      {showLoginPopup && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center p-4 z-50">
          <div className="bg-[#1e1f20] border border-[#2f3032] rounded-2xl p-8 max-w-md w-full relative shadow-2xl">
            <button
              onClick={() => setShowLoginPopup(false)}
              className="absolute top-5 right-5 text-[#8e918f] hover:text-[#e3e3e3] text-xl leading-none cursor-pointer"
            >
              ✕
            </button>
            <div className="text-center">
              {/* <div className="w-12 h-12 rounded-full bg-[#282a2c] text-[#a8c7fa] flex items-center justify-center text-xl mx-auto mb-4"> */}
              <img
                src="logo.png"
                alt="Yuuna Logo"
                className="w-13 h-13 flex items-center justify-center text-xl mx-auto mb-4"
              />
              {/* </div> */}
              <h2 className="text-lg font-semibold text-[#e3e3e3] mb-2">
                เข้าสู่ระบบเพื่อคุยต่อ
              </h2>
              <p className="text-[#8e918f] text-sm mb-6 leading-relaxed">
                คุณได้สนทนาครบโควตาทดลองแล้ว เข้าสู่ระบบด้วย Google
                เพื่อแชทต่อและบันทึกความทรงจำกับยูนะ
              </p>
              <div className="flex justify-center my-2">
                <GoogleLogin
                  onSuccess={handleGoogleSuccess}
                  onError={() => console.log("Login Failed")}
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
