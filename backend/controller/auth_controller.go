package controller

import (
	"context"
	"strings"
	"time"

	"yuuna/config"
	"yuuna/database"
	"yuuna/model"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/idtoken"
)

const sessionCookieName = "token"
const sessionTTL = time.Hour * 72

// userPayload แปลง model.User เป็นรูปแบบที่ส่งกลับให้ Frontend (ไม่มี token ใน JSON)
func userPayload(user model.User) fiber.Map {
	return fiber.Map{
		"id":      user.ID,
		"name":    user.Name,
		"email":   user.Email,
		"picture": user.Picture,
	}
}

// setSessionCookie ออก JWT ผ่าน HttpOnly Cookie เท่านั้น (ไม่ส่ง token ใน body)
func setSessionCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Expires:  time.Now().Add(sessionTTL),
		MaxAge:   int(sessionTTL.Seconds()),
		HTTPOnly: true,           // JavaScript อ่าน/แก้ไขไม่ได้ ป้องกัน XSS
		Secure:   true,           // ส่งผ่าน HTTPS เท่านั้น
		SameSite: "Lax",          // ป้องกัน CSRF และยังส่ง cookie ได้เมื่อ Frontend/Backend อยู่คนละ subdomain เดียวกัน
		Path:     "/",
	})
}

// clearSessionCookie ทำลาย Session Cookie ฝั่งเซิร์ฟเวอร์
func clearSessionCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		Path:     "/",
	})
}

// GoogleLogin ตรวจสอบ Google ID Token กับ Google ก่อนเสมอ แล้วจึงออก Session Cookie
func GoogleLogin(c *fiber.Ctx) error {
	var req model.GoogleAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if strings.TrimSpace(req.IDToken) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "ID Token is required"})
	}

	// 1. ตรวจสอบ ID Token กับทาง Google (บังคับ Audience = GOOGLE_CLIENT_ID ของเซิร์ฟเวอร์)
	clientID := config.GetEnv("GOOGLE_CLIENT_ID", "")
	if clientID == "" {
		return c.Status(500).JSON(fiber.Map{"error": "Google sign-in is not configured"})
	}

	payload, err := idtoken.Validate(context.Background(), req.IDToken, clientID)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid Google token"})
	}

	// 2. ดึงข้อมูลจาก Payload ที่ตรวจสอบแล้ว (ใช้ type assertion แบบปลอดภัย ป้องกัน panic)
	googleID := payload.Subject
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	if googleID == "" || email == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Google token missing required claims"})
	}
	if name == "" {
		name = email
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

	// 4. สร้าง JWT ของระบบแล้วส่งกลับผ่าน HttpOnly Cookie เท่านั้น
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(sessionTTL).Unix(),
	})

	secret := config.GetEnv("JWT_SECRET", "")
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	setSessionCookie(c, t)

	return c.JSON(fiber.Map{
		"message": "Login successful",
		"user":    userPayload(user),
	})
}

// GetMe ตรวจสอบสถานะล็อกอินจาก Session Cookie และคืนข้อมูลโปรไฟล์ปัจจุบัน
// เรียกผ่าน middleware.Protected() ถ้า Cookie ไม่ถูกต้อง/หมดอายุ จะได้ 401 จาก middleware ก่อน
func GetMe(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var user model.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		clearSessionCookie(c) // ผู้ใช้ถูกลบไปแล้ว ให้ล้าง cookie ทิ้งด้วย
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	return c.JSON(fiber.Map{"user": userPayload(user)})
}

// Logout ทำลาย Session Cookie ฝั่งเซิร์ฟเวอร์ (ไม่ต้องผ่าน middleware เพื่อให้ล้างได้เสมอ)
func Logout(c *fiber.Ctx) error {
	clearSessionCookie(c)
	return c.JSON(fiber.Map{"message": "Logged out successfully"})
}
