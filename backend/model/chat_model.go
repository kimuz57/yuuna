package model

import "gorm.io/gorm"

// ChatMessage เก็บประวัติการแชท
type ChatMessage struct {
	gorm.Model
	UserID  uint   `json:"user_id" gorm:"index;not null"` // ใช้ UserID แทน session_id
	Role    string `json:"role" gorm:"not null"`
	Content string `json:"content" gorm:"not null"`
}



type ChatRequest struct {
	Message string `json:"message"`
}
type GuestChatRequest struct {
	Message string              `json:"message"`
	History []ChatMessage       `json:"history"` // ประวัติที่ดึงมาจาก LocalStorage 
}