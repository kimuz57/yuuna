package controller

import (
	"context"
	"time"

	"yuuna/config"
	"yuuna/database"
	"yuuna/model"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/idtoken"
	
)

// GoogleLogin ตรวจสอบ Google ID Token และออก JWT ของระบบเรา
func GoogleLogin(c *fiber.Ctx) error {
	var req model.GoogleAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.IDToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "ID Token is required"})
	}

	// 1. ตรวจสอบ ID Token กับทาง Google
	// ต้องใส่ Google Client ID ของคุณในไฟล์ .env (เช่น GOOGLE_CLIENT_ID=xxx.apps.googleusercontent.com)
	clientID := config.GetEnv("GOOGLE_CLIENT_ID", "") 
	payload, err := idtoken.Validate(context.Background(), req.IDToken, clientID)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid Google token"})
	}

	// 2. ดึงข้อมูลจาก Payload ที่ตรวจสอบแล้ว
	googleID := payload.Subject // รหัส ID ของผู้ใช้จาก Google
	email := payload.Claims["email"].(string)
	name := payload.Claims["name"].(string)
	picture := ""
	if pic, ok := payload.Claims["picture"].(string); ok {
		picture = pic
	}

	// 3. ค้นหาผู้ใช้ใน Database ถ้าไม่มีให้สร้างใหม่ (Upsert)
	var user model.User
	result := database.DB.Where("google_id = ?", googleID).First(&user)
	
	if result.Error != nil { // ถ้าไม่พบผู้ใช้ ให้สมัครสมาชิกใหม่โดยอัตโนมัติ
		user = model.User{
			GoogleID: googleID,
			Email:    email,
			Name:     name,
			
			Picture:  picture,
		}
		if err := database.DB.Create(&user).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
		}
	}

	// 4. สร้าง JWT Token ของระบบเราเอง
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	})

	secret := config.GetEnv("JWT_SECRET", "super-secret-key")
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	// 🍪 ตั้งค่า JWT ลงใน HttpOnly Cookie (ปลอดภัยจาก XSS)
	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    t,
		Expires:  time.Now().Add(time.Hour * 72),
		HTTPOnly: true,  // JavaScript อ่านไม่ได้ ปลอดภัย 100%
		Secure:   true, // ตั้งเป็น true ถ้าขึ้น Production ที่ใช้ HTTPS
		SameSite: "none", // SameSite=None เพื่อให้สามารถส่ง Cookie ข้าม Domain ได้ (เช่น Frontend กับ Backend อยู่คนละ Domain)
		Path:     "/",
	})

	// ส่งกลับไปแค่ข้อมูล User (ไม่ต้องส่ง token มาที่ JSON แล้ว)
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"user": fiber.Map{
			"id":      user.ID,
			"name":    user.Name,
			"email":   user.Email,
			"picture": user.Picture,
		},
	})
}