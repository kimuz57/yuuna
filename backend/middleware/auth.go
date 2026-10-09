package middleware

import (
	"yuuna/config"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Protected เป็น Middleware ตรวจสอบ Session JWT จาก HttpOnly Cookie เท่านั้น
// สถานะล็อกอินถูกตัดสินใจโดยเซิร์ฟเวอร์เท่านั้น — ไม่เชื่อข้อมูลใด ๆ จาก localStorage ของฝั่ง Client
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. 🍪 ดึง Token จาก HttpOnly Cookie (JavaScript ฝั่งผู้ใช้อ่านไม่ได้)
		tokenString := c.Cookies("token")

		// ถ้าไม่มี Cookie แปลว่ายังไม่ได้ล็อกอิน
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Missing session cookie",
			})
		}

		secret := config.GetEnv("JWT_SECRET", "")
		if secret == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Server misconfiguration: JWT_SECRET is not set",
			})
		}

		// 2. Parse และ Validate Token (บังคับ HMAC เท่านั้น ป้องกันการสลับ algorithm)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid or expired token",
			})
		}

		// 3. ดึงข้อมูล Claims ออกมา
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid token claims",
			})
		}

		// 4. ดึง user_id ไปใช้งานต่อใน Controller
		if idFloat, ok := claims["user_id"].(float64); ok {
			c.Locals("user_id", uint(idFloat))
		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid user ID in token",
			})
		}

		return c.Next()
	}
}
