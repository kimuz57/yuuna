package routes

import (
	"time"
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
	// ประกาศ Group แค่ครั้งเดียวพอครับ
	api := app.Group("/api", middleware.Protected())
	{
		// 🟢 1. จัดการห้องแชท (Sidebar)
		api.Get("/sessions", controller.GetSessions)

		api.Put("/sessions/:id", controller.UpdateSession)     // หรือชื่อฟังก์ชันที่คุณตั้งใน controller
		api.Delete("/sessions/:id", controller.DeleteSession) // หรือชื่อฟังก์ชันที่คุณตั้งใน controller
		
		// 🟢 2. จัดการแชทและประวัติ
		api.Post("/chat", controller.HandleChat)
		api.Get("/history", controller.GetHistory)
		api.Delete("/history", controller.ClearHistory)

		// 🟢 3. จัดการเรื่อง Migrate ประวัติแชท (เอา Protected ออกเพราะ Group บังคับไปแล้ว)
		api.Post("/history/migrate", controller.MigrateHistory)

		// 🟢 4. API ล็อกเอาต์ (ล้าง HttpOnly Cookie)
		api.Post("/auth/logout", func(c *fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:     "jwt", // เปลี่ยนให้ตรงกับชื่อ Cookie ที่คุณตั้งไว้ตอน Login นะครับ
				Value:    "",
				Expires:  time.Now().Add(-time.Hour),
				HTTPOnly: true,
				SameSite: "Lax",
			})
			return c.JSON(fiber.Map{"message": "Logged out successfully"})
		})
	}
}