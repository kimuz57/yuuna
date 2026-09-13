package model

import "gorm.io/gorm"

// User เก็บข้อมูลบัญชีผู้ใช้งาน
type User struct {
	gorm.Model
	GoogleID string `json:"google_id" gorm:"uniqueIndex;not null"` // รหัสเฉพาะจาก Google
	Email    string `json:"email" gorm:"uniqueIndex;not null"`
	Name     string `json:"name"`                                  // ชื่อผู้ใช้จาก Google
	Picture  string `json:"picture"`                               // รูปโปรไฟล์ (เผื่อใช้แสดงผล)
	Password string `json:"password,omitempty"`                    // เพิ่มฟิลด์ Password รองรับกรณีสมัครแบบปกติและปล่อยว่างได้สำหรับ Google Login
	
	Messages []ChatMessage `json:"messages,omitempty" gorm:"foreignKey:UserID"`
}

// Struct สำหรับรับค่าจาก Frontend
type GoogleAuthRequest struct {
	IDToken string `json:"id_token"`
}

// Struct สำหรับรับข้อมูล Login/Register
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}