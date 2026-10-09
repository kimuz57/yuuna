package routes

import (
	"yuuna/controller"
	"yuuna/middleware"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes จัดการลงทะเบียน Endpoint ทั้งหมดของแอปพลิเคชัน
func SetupRoutes(app *fiber.App) {
	// ==========================================
	// Public Routes (ไม่ต้องมี Session Cookie)
	// ==========================================
	app.Post("/api/guest/chat", controller.HandleGuestChat)

	// Auth สาธารณะ: ล็อกอิน (verify Google token แล้วออก HttpOnly Cookie)
	// และล็อกเอาต์ (ทำลาย HttpOnly Cookie) — ล็อกเอาต์ต้องเรียกได้เสมอ แม้ cookie หมดอายุ
	app.Post("/api/auth/google", controller.GoogleLogin)
	app.Post("/api/auth/logout", controller.Logout)

	// ==========================================
	// Protected Routes (ต้องมี Session Cookie ที่ถูกต้อง)
	// ==========================================
	// ทุกเส้นทางในกลุ่มนี้ถูกคุ้มครองด้วย middleware.Protected() ก่อนเสมอ
	api := app.Group("/api", middleware.Protected())
	{
		// 🟢 0. ตรวจสอบสถานะล็อกอินจาก Cookie (คืน 401 ถ้าไม่ได้ล็อกอิน/หมดอายุ)
		api.Get("/auth/me", controller.GetMe)

		// 🟢 1. จัดการห้องแชท (Sidebar)
		api.Get("/sessions", controller.GetSessions)

		api.Put("/sessions/:id", controller.UpdateSession)
		api.Delete("/sessions/:id", controller.DeleteSession)

		// 🟢 2. จัดการแชทและประวัติ
		api.Post("/chat", controller.HandleChat)
		api.Get("/history", controller.GetHistory)
		api.Delete("/history", controller.ClearHistory)

		// 🟢 3. จัดการเรื่อง Migrate ประวัติแชท
		api.Post("/history/migrate", controller.MigrateHistory)
	}
}
