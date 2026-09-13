package controller

import (
	"strings"
	"time"
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	
	"yuuna/core"
	"yuuna/model"
	"yuuna/store"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// ฟังก์ชันดึง IP ที่แท้จริงของผู้ใช้
func getRealIP(c *fiber.Ctx) string {
	// 1. ลองดึงจาก Header ที่ Proxy/Tunnel ส่งมาให้ก่อน
	forwardedFor := c.Get("X-Forwarded-For")
	if forwardedFor != "" {
		// บางครั้งมันจะส่งมาเป็นชุด (เช่น "172.16.0.2, 127.0.0.1") เราต้องเอาตัวแรกสุด
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// 2. ถ้าไม่ได้ใช้ Tunnel ก็ดึงจาก Connection ปกติ
	return c.IP()
}

// HandleGuestChat สำหรับผู้ใช้ที่ยังไม่ได้ Login (ไม่บันทึกลง DB/Redis)
func HandleGuestChat(c *fiber.Ctx) error {
	var req model.GuestChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Message == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Message is required"})
	}

	// ==========================================
	// 🛡️ ระบบป้องกันด้วย Redis + IP Address
	// ==========================================
	clientIP := getRealIP(c)
	quotaKey := "guest:quota:" + clientIP

	// เช็คจำนวนข้อความที่เคยส่งมา
	count, err := store.RedisClient.Get(c.Context(), quotaKey).Int()
	if err != nil && err != redis.Nil {
		log.Println("Redis Error:", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	// ถ้าครบ 5 ข้อความแล้ว ดีดกลับทันที ไม่ให้คุยกับ AI
	if count >= 5 {
		return c.Status(403).JSON(fiber.Map{
			"error":   "QUOTA_EXCEEDED",
			"message": "คุณใช้งานโควตาทดลองครบ 5 ข้อความแล้ว กรุณาเข้าสู่ระบบ",
		})
	}

	// ถ้ายังไม่ครบ ให้เพิ่มตัวนับ +1 
	// และตั้งเวลาหมดอายุ (TTL) ไว้ที่ 24 ชั่วโมง เพื่อไม่ให้ข้อมูลขยะเต็ม Redis
	store.RedisClient.Incr(c.Context(), quotaKey)
	store.RedisClient.Expire(c.Context(), quotaKey, 24*time.Hour)
	// ==========================================

	reqCtx := c.UserContext()

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")


	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		modelAI := core.GeminiClient.GenerativeModel(MODEL_NAME)
		modelAI.SetMaxOutputTokens(2048)
		modelAI.SetTemperature(0.7)

		modelAI.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(core.YuunaSystemPrompt)},
		}

		var genaiHistory []*genai.Content
		for _, msg := range req.History {
			role := msg.Role
			if role == "ai" || role == "model" {
				role = "model"
			} else {
				role = "user"
			}
			genaiHistory = append(genaiHistory, &genai.Content{
				Role:  role,
				Parts: []genai.Part{genai.Text(msg.Content)},
			})
		}

		cs := modelAI.StartChat()
		cs.History = genaiHistory

		// ใช้ reqCtx ที่ดึงไว้ล่วงหน้า
		iter := cs.SendMessageStream(reqCtx, genai.Text(req.Message))
		
		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Println("Guest stream error:", err)
				errChunk, _ := json.Marshal(fiber.Map{"error": true, "message": "stream interrupted"})
				fmt.Fprintf(w, "data: %s\n\n", errChunk)
				w.Flush()
				break
			}

			for _, cand := range resp.Candidates {
				if cand.Content != nil {
					for _, part := range cand.Content.Parts {
						text := fmt.Sprintf("%v", part)
						chunk, _ := json.Marshal(fiber.Map{"text": text})
						fmt.Fprintf(w, "data: %s\n\n", chunk)
						w.Flush()
					}
				}
			}
		}

		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.Flush()
	})

	return nil
}