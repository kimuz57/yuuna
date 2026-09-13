package controller

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"yuuna/database" // 🟢 เรียกใช้แพ็กเกจฐานข้อมูลของคุณ
	"yuuna/model"     // 🟢 เรียกใช้โมเดล ChatSession
)

type UpdateSessionRequest struct {
	Title string `json:"title"`
}

// ---------------------------------------------------------
// 1. API สำหรับแก้ไขชื่อแชท (PUT /api/sessions/:id)
// ---------------------------------------------------------
func UpdateSession(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	
	userID := c.Locals("user_id") // ปรับชื่อ Key ตาม Middleware ของคุณ
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req UpdateSessionRequest
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body or empty title"})
	}

	// 🌟 สั่งอัปเดตชื่อใน Database จริงๆ ด้วย GORM (เช็กทั้ง id และ user_id เพื่อความปลอดภัย)
	result := database.DB.Model(&model.ChatSession{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Update("title", req.Title)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update session in database"})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Session not found or unauthorized"})
	}

	return c.JSON(fiber.Map{"message": "Session updated successfully", "title": req.Title})
}

// ---------------------------------------------------------
// 2. API สำหรับลบแชท (DELETE /api/sessions/:id)
// ---------------------------------------------------------
func DeleteSession(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	
	userID := c.Locals("user_id")
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// 🌟 สั่งลบข้อความในห้องแชท และตัวห้องแชทออกจาก Database จริงๆ ด้วย GORM
	// 1. ลบข้อความทั้งหมดที่อยู่ใน session นี้ก่อน
	if err := database.DB.Where("session_id = ? AND user_id = ?", sessionID, userID).Delete(&model.ChatMessage{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete chat messages"})
	}

	// 2. ลบตัวห้องแชทหลัก
	result := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&model.ChatSession{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete session"})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Session not found or unauthorized"})
	}

	return c.JSON(fiber.Map{"message": "Session deleted successfully"})
}