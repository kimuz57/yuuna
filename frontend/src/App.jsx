import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { GoogleOAuthProvider } from '@react-oauth/google';

import Chat from './pages/Chat';
import Kol from './pages/Kol';

// ดึงค่าจาก Env ของ Vite
const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID;

export default function App() {
  return (
    <GoogleOAuthProvider clientId={GOOGLE_CLIENT_ID}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Chat />} />
          <Route path="/kol" element={<Kol />} />
          <Route path="*" element={<Chat />} />
        </Routes>
      </BrowserRouter>
    </GoogleOAuthProvider>
  );
}