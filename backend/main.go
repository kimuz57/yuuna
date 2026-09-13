package main

import (
	"log"

	"yuuna/config"
	"yuuna/core"
	"yuuna/database"
	"yuuna/routes"
	"yuuna/store"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	config.LoadEnv()
	FRONTEND_URL := config.GetEnv("FRONTEND_URL", "http://localhost:5173")
	database.ConnectDB(config.GetEnv("DATABASE_URL", "")) //[cite: 5]

	redisAddr := config.GetEnv("REDIS_ADDR", "localhost:6379") //[cite: 5]
	redisPassword := config.GetEnv("REDIS_PASSWORD", "") //[cite: 5]
	if err := store.InitRedis(redisAddr, redisPassword, 0); err != nil { //[cite: 4, 5]
		log.Fatalf("❌ Redis connection failed: %v", err) //[cite: 5]
	}
	log.Println("⚡ Redis connected successfully!") //[cite: 5]

	core.InitGemini(config.GetEnv("GEMINI_API_KEY", "")) //[cite: 5]

	app := fiber.New() //[cite: 5]
	app.Use(cors.New(cors.Config{
		AllowOrigins:     FRONTEND_URL, // ระบุเจาะจงที่อยู่ของ Frontend
		AllowCredentials: true,                    // อนุญาตให้ส่ง Cookie ผ่านได้
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// ลงทะเบียน Routes ทั้งหมด
	routes.SetupRoutes(app)

	port := config.GetEnv("PORT", "8080") //[cite: 5]
	log.Printf("🚀 Server running on port %s", port) //[cite: 5]
	log.Fatal(app.Listen(":" + port)) //[cite: 5]
}