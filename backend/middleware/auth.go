package middleware

import (
	"strings"
	"yuuna/config"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Protected เป็น Middleware ตรวจสอบ JWT Token จาก HttpOnly Cookie
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. 🍪 ดึง Token จาก HttpOnly Cookie โดยตรง
		tokenString := c.Cookies("token")

		// (เผื่อไว้) ถ้าหาใน Cookie ไม่เจอ ลองเช็คใน Header แบบ Bearer เผื่อมีบางจุดยังใช้แบบเก่า
		if tokenString == "" {
			authHeader := c.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		// ถ้ายังหาไม่เจอทั้งสองที่ แปลว่ายังไม่ได้ล็อกอิน
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Missing token in cookie",
			})
		}

		secret := config.GetEnv("JWT_SECRET", "super-secret-key")

		// Parse และ Validate Token
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

		// ดึงข้อมูล Claims ออกมา
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized: Invalid token claims",
			})
		}

		// ดึง user_id ไปใช้งานต่อใน Controller
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