package main

import (
	"log"
	"strings"

	"yuuna/config"
	"yuuna/core"
	"yuuna/database"
	"yuuna/routes"
	"yuuna/store"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// loopbackOnly บังคับให้ Bind เฉพาะ localhost เท่านั้น
// ป้องกันไม่ให้พอร์ตของเซิร์ฟเวอร์ถูกเปิดรับการเชื่อมต่อจากเครือข่ายภายนอกโดยตรง
func loopbackOnly(host string) string {
	switch strings.TrimSpace(host) {
	case "127.0.0.1", "localhost", "::1":
		return strings.TrimSpace(host)
	default:
		log.Printf("⚠️  HOST=%q ไม่ใช่ loopback — บังคับใช้ 127.0.0.1 เพื่อไม่เปิดรับจากภายนอก", host)
		return "127.0.0.1"
	}
}

// allowedOrigins รวมรายการ Origin ที่อนุญาต (คั่นด้วย ,) จาก ALLOWED_ORIGINS + FRONTEND_URL
// อนุญาตเฉพาะโดเมนของระบบเท่านั้น ไม่ใช้ wildcard
func allowedOrigins() string {
	defaults := []string{"https://yuuna.tcmg.me", "http://localhost:3000"}

	seen := map[string]bool{}
	var origins []string
	add := func(o string) {
		o = strings.TrimSuffix(strings.TrimSpace(o), "/")
		if o != "" && !seen[o] {
			seen[o] = true
			origins = append(origins, o)
		}
	}

	for _, o := range strings.Split(config.GetEnv("ALLOWED_ORIGINS", ""), ",") {
		add(o)
	}
	add(config.GetEnv("FRONTEND_URL", "")) // origin ของ Frontend ที่ตั้งไว้ใน .env
	for _, o := range defaults {
		add(o)
	}
	return strings.Join(origins, ",")
}

func main() {
	config.LoadEnv()

	// Fail-fast: ค่าที่จำเป็นต่อความปลอดภัยของ Auth ต้องถูกตั้งค่าเสมอ
	// (ถ้าขาด ระบบจะยังตรวจ JWT ด้วยค่า default ที่ทายได้ หรือข้ามการตรวจสอบ Google Audience)
	if config.GetEnv("JWT_SECRET", "") == "" {
		log.Fatal("❌ JWT_SECRET is not set. Generate one (e.g. `openssl rand -hex 32`) and add it to backend/.env")
	}
	if config.GetEnv("GOOGLE_CLIENT_ID", "") == "" {
		log.Fatal("❌ GOOGLE_CLIENT_ID is not set. Add the Google OAuth Client ID to backend/.env")
	}

	database.ConnectDB(config.GetEnv("DATABASE_URL", ""))

	redisAddr := config.GetEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := config.GetEnv("REDIS_PASSWORD", "")
	if err := store.InitRedis(redisAddr, redisPassword, 0); err != nil {
		log.Fatalf("❌ Redis connection failed: %v", err)
	}
	log.Println("⚡ Redis connected successfully!")

	core.InitGemini(config.GetEnv("GEMINI_API_KEY", ""))

	// Trusted Proxies: อ่าน IP ที่แท้จริงจาก X-Forwarded-For ก็ต่อเมื่อ request
	// วิ่งมาจาก proxy บนเครื่องเดียวกันเท่านั้น (Tunnel/Vite proxy) ป้องกันการ Spoof หัวข้อจากภายนอก
	trustedProxies := []string{"127.0.0.1", "::1"}
	if envProxies := config.GetEnv("TRUSTED_PROXIES", ""); envProxies != "" {
		trustedProxies = nil
		for _, p := range strings.Split(envProxies, ",") {
			if p = strings.TrimSpace(p); p != "" {
				trustedProxies = append(trustedProxies, p)
			}
		}
	}

	app := fiber.New(fiber.Config{
		ProxyHeader:             "X-Forwarded-For",
		EnableTrustedProxyCheck: true,
		TrustedProxies:          trustedProxies,
	})

	// CORS: อนุญาตเฉพาะโดเมนของระบบเท่านั้น (ไม่ใช้ * เพราะเปิด CORS แบบ Credentials)
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins(),
		AllowCredentials: true, // จำเป็นสำหรับการส่ง HttpOnly Cookie
		AllowHeaders:     "Origin, Content-Type, Accept",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// ลงทะเบียน Routes ทั้งหมด
	routes.SetupRoutes(app)

	// Bind เฉพาะ 127.0.0.1 เท่านั้น — ห้ามใช้ :8080 หรือ 0.0.0.0:8080
	port := config.GetEnv("PORT", "8080")
	host := loopbackOnly(config.GetEnv("HOST", "127.0.0.1"))
	log.Printf("🚀 Server running on http://%s:%s (localhost only)", host, port)
	log.Fatal(app.Listen(host + ":" + port))
}
