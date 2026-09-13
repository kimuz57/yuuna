package routes

import (
	"yuuna/controller"
	"yuuna/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes จัดการลงทะเบียน Endpoint ทั้งหมดของแอปพลิเคชัน
func SetupRoutes(app *fiber.App) {
	// ==========================================
	// Public Routes (ไม่ต้องใช้ Token)
	// ==========================================
	app.Post("/api/guest/chat", controller.HandleGuestChat)
	app.Post("/api/auth/google", controller.GoogleLogin)

	// ==========================================
	// Protected Routes (ต้องยืนยันตัวตนด้วย JWT)
	// ==========================================
	api := app.Group("/api", middleware.Protected())

	api.Post("/chat", controller.HandleChat)
	api.Get("/history", controller.GetHistory)
	api.Delete("/history", controller.ClearHistory)
	api.Post("/history/migrate", middleware.Protected(), controller.MigrateHistory)
}