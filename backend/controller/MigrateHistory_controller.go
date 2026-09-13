package controller

import (
	"log"
	"yuuna/database"
	"yuuna/model"

	"github.com/gofiber/fiber/v2"
)

func MigrateHistory(c *fiber.Ctx) error {
	// 1. ดึง User ID และดักจับความปลอดภัยกันแอปเด้ง (Panic)
	userIDVal := c.Locals("user_id")
	if userIDVal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// แปลง Type ให้ปลอดภัย (JWT ส่วนใหญ่มักจะคืนค่าเป็น float64)
	var userID uint
	switch v := userIDVal.(type) {
	case uint:
		userID = v
	case float64:
		userID = uint(v)
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid User ID format"})
	}

	// 2. รับค่าจาก Request Body
	var req struct {
		History []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"history"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	// ถ้าไม่มีประวัติแชทส่งมาเลย ก็ให้ข้ามไปเลย ไม่ต้องเปลืองแรง DB
	if len(req.History) == 0 {
		return c.JSON(fiber.Map{"message": "No history to migrate"})
	}

	// 3. เตรียมข้อมูลเป็นก้อน (Slice) เพื่อทำ Batch Insert
	var chatMessages []model.ChatMessage
	for _, h := range req.History {
		chatMessages = append(chatMessages, model.ChatMessage{
			UserID:  userID,
			Role:    h.Role,
			Content: h.Content,
		})
	}

	// 4. บันทึกลง Database ทีเดียวรวดเดียว (Batch Insert) เร็วกว่าลูป Create หลายเท่า!
	if err := database.DB.Create(&chatMessages).Error; err != nil {
		log.Printf("❌ MigrateHistory DB Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save history to database"})
	}

	return c.JSON(fiber.Map{"message": "History migrated successfully"})
}