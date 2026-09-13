import React from 'react';
import { Link } from 'react-router-dom';

export default function Kol() {
  return (
    <div className="flex flex-col items-center justify-center h-screen bg-gray-100">
      <h1 className="text-3xl font-bold mb-4">หน้า KOL</h1>
      <p className="text-gray-600 mb-6">หน้านี้เตรียมไว้สำหรับการพัฒนาในอนาคตครับ</p>
      
      {/* ใช้ Link แทนแท็ก <a> เพื่อไม่ให้หน้าเว็บโหลดใหม่ (กระตุก) */}
      <Link to="/" className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        กลับไปหน้าแชท
      </Link>
    </div>
  );
}