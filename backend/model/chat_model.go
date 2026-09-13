package model

import "gorm.io/gorm"

// 1. สร้าง Table ใหม่สำหรับเก็บ "ห้องแชท" (Sidebar)
type ChatSession struct {
	gorm.Model
	UserID   uint          `json:"user_id" gorm:"index;not null"`
	Title    string        `json:"title" gorm:"default:'แชทใหม่กับยูนะ'"` // ชื่อห้องแชท (แสดงที่ Sidebar)
	Messages []ChatMessage `json:"messages" gorm:"foreignKey:SessionID"` // 1 ห้อง มีหลายข้อความ
}

// 2. อัปเดต Table ข้อความ ให้ผูกกับห้องแชท (SessionID)
type ChatMessage struct {
	gorm.Model
	SessionID uint   `json:"session_id" gorm:"index"` // เพิ่มฟิลด์นี้เพื่อให้รู้ว่าข้อความนี้อยู่ห้องไหน
	UserID    uint   `json:"user_id" gorm:"index;not null"`
	Role      string `json:"role" gorm:"not null"`
	Content   string `json:"content" gorm:"not null"`
}

// 3. อัปเดต Request ที่รับจากหน้าบ้าน ให้ส่ง SessionID มาด้วย
type ChatRequest struct {
	SessionID *uint   `json:"session_id"` // ต้องรู้ว่ากำลังพิมพ์ลงห้องไหน
	Message   string `json:"message"`
}

type GuestChatRequest struct {
	Message string        `json:"message"`
	History []ChatMessage `json:"history"`
}