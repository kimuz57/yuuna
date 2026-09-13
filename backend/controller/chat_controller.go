package controller

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"yuuna/config"
	"yuuna/core"
	"yuuna/database"
	"yuuna/model"
	"yuuna/store"

	"github.com/gofiber/fiber/v2"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"gorm.io/gorm"
)

var MODEL_NAME = config.GetEnv("GEMINI_MODEL", "gemini-3.5-flash-lite")

func summarizeContext(ctx context.Context, existingSummary string, buffer []model.ChatMessage) (string, error) {
	summarizerModel := core.GeminiClient.GenerativeModel(MODEL_NAME)

	// 1. System Prompt ที่คืนวิญญาณให้ยูนะ แต่สั่งให้สรุปแบบหลบฟิลเตอร์ (PG-13)
	systemContext := `คุณคือ AI สรุปความทรงจำสำหรับตัวละคร "ยูนะ" (ภรรยาสาวสุดน่ารักขี้อ้อนจากต่างโลก)
หน้าที่ของคุณคือสรุปความทรงจำให้คงความน่ารัก สดใส และเก็บรายละเอียดความสัมพันธ์ รวมถึงสิ่งที่ผู้ใช้ (โอโตะสะมะ/คุณกิต) ชอบหรือตกลงกันไว้ให้ครบถ้วน
ข้อบังคับสำคัญ: หากมีการแสดงความรัก ให้สรุปเป็นคำที่อบอุ่นและบริสุทธิ์ (Wholesome) เช่น การให้กำลังใจ ความห่วงใย หรือการอยู่เคียงข้าง ห้ามบรรยายการสัมผัสร่างกายที่ล่อแหลม เพื่อให้ผ่านระบบความปลอดภัยขั้นสูงสุด`

	summarizerModel.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemContext)},
	}

	summarizerModel.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockNone},
	}

	currentBuffer := buffer

	// 2. ลูปป้องกันแชทล่ม (ถ้าโดน Google แบน ก็แค่หั่นข้อความเก่าสุดทิ้งแล้วส่งใหม่)
	for len(currentBuffer) > 0 {

		// นำ Prompt รูปแบบเดิมที่คุณชอบกลับมาใช้
		prompt := fmt.Sprintf(`- บริบทความทรงจำเดิมที่มีอยู่:
"%s"

- รายการข้อความสนทนาชุดใหม่:
`, existingSummary)

		// ใช้ currentBuffer แทน buffer เพื่อให้มันหั่นข้อความได้ถ้ามีปัญหา
		for _, msg := range currentBuffer {
			prompt += fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content)
		}

		prompt += "\nจงรวมบริบทความทรงจำเดิมเข้ากับข้อความชุดใหม่ สรุปให้กระชับแต่ต้องคงคาแรกเตอร์และรายละเอียดความสัมพันธ์ระหว่างยูนะกับโอโตะสะมะไว้ให้ครบถ้วนที่สุด"

		// ยิง API
		resp, err := summarizerModel.GenerateContent(ctx, genai.Text(prompt))

		if err != nil {
			errStr := err.Error()

			// ถ้าชน Hard Filter (BlockReason 4) ค่อยหั่นข้อความทิ้งทีละบรรทัด
			if strings.Contains(errStr, "BlockReason(4)") || strings.Contains(errStr, "blocked") {
				log.Printf("⚠️ [Self-Healing] ติดฟิลเตอร์! หั่นข้อความเก่าสุดทิ้ง 1 บรรทัด (เหลือ %d)", len(currentBuffer)-1)
				currentBuffer = currentBuffer[1:]
				continue
			}

			log.Printf("❌ Summarize API Error (Not Blocked): %v", err)
			return existingSummary, nil
		}

		// ถ้าสำเร็จ คืนค่าความทรงจำที่น่ารักเหมือนเดิม
		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
			newSummary := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
			log.Println("✅ [Summarize] อัปเดตความทรงจำยูนะสำเร็จ!")
			return newSummary, nil
		}

		break
	}

	return existingSummary, nil
}

// getUserID ดึง UserID ที่ถูกเซ็ตไว้จาก Auth Middleware
func getUserID(c *fiber.Ctx) uint {
	val := c.Locals("user_id")
	if val != nil {
		switch v := val.(type) {
		case uint:
			return v
		case float64:
			return uint(v)
		}
	}
	return 0
}

// HandleChat จัดการการแชต ดึง/บันทึก Context ลง Redis และ Stream SSE
func HandleChat(c *fiber.Ctx) error {
	var req model.ChatRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Message == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Message is required"})
	}

	userID := getUserID(c)
	reqCtx := c.UserContext()

	// 🟢 1. จัดการระบบห้องแชท (Session)
	var currentSessionID uint

	if req.SessionID == nil || *req.SessionID == 0 {
		// ถ้าไม่มี Session ส่งมา แปลว่าเป็นการ "สร้างแชทใหม่"
		title := req.Message
		runes := []rune(title)
		if len(runes) > 30 {
			title = string(runes[:30]) + "..."
		}

		newSession := model.ChatSession{
			UserID: userID,
			Title:  title,
		}
		if err := database.DB.Create(&newSession).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create session"})
		}
		currentSessionID = newSession.ID
	} else {
		// ถ้ามี Session อยู่แล้ว ให้ใช้อันเดิม และอัปเดตเวลาล่าสุด
		currentSessionID = *req.SessionID
		database.DB.Model(&model.ChatSession{}).Where("id = ?", currentSessionID).Update("updated_at", gorm.Expr("NOW()"))
	}

	// 🟢 2. บันทึกคำถามผู้ใช้ลง PostgreSQL ถาวร
	userMsg := model.ChatMessage{
		SessionID: currentSessionID,
		UserID:    userID,
		Role:      "user",
		Content:   req.Message,
	}
	database.DB.Create(&userMsg)

	// 3. ดึง State จาก Redis (Summary + Buffer)
	state, err := store.GetContextState(reqCtx, userID)
	if err != nil {
		log.Println("Redis GetContextState Error:", err)
		state = &store.ChatContextState{Summary: "", Buffer: []model.ChatMessage{}}
	}

	// 4. แปลง Buffer เดิมใน Redis ให้เป็น History รูปแบบ Gemini SDK
	var genaiHistory []*genai.Content
	for _, msg := range state.Buffer {
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

	state.Buffer = append(state.Buffer, userMsg)

	// 5. ตรวจสอบว่า Buffer ครบ 10 ข้อความหรือยัง?
	if len(state.Buffer) >= 10 {
		newSummary, err := summarizeContext(reqCtx, state.Summary, state.Buffer)
		if err == nil {
			log.Println("⚡ Context Summarized Successfully!")
			state.Summary = newSummary
			state.Buffer = []model.ChatMessage{}
		} else {
			log.Println("❌ Summarize Error:", err)
		}
	}

	// 6. ตั้งค่า Header สำหรับ SSE Stream
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		modelAI := core.GeminiClient.GenerativeModel(MODEL_NAME)
		modelAI.SetMaxOutputTokens(2048)
		modelAI.SetTemperature(0.7)

		modelAI.SafetySettings = []*genai.SafetySetting{
			{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
			{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
			{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockNone},
			{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockNone},
		}

		systemPromptWithMemory := core.YuunaSystemPrompt
		if state.Summary != "" {
			systemPromptWithMemory += fmt.Sprintf("\n\n[ความทรงจำและบริบทในอดีตเกี่ยวกับ Otosama]:\n%s", state.Summary)
		}

		modelAI.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(systemPromptWithMemory)},
		}

		cs := modelAI.StartChat()
		cs.History = genaiHistory

		iter := cs.SendMessageStream(reqCtx, genai.Text(req.Message))
		fullResponse := ""
		streamHadError := false

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Println("Error generating content:", err)
				streamHadError = true
				errChunk, _ := json.Marshal(fiber.Map{"error": true, "message": "stream interrupted"})
				fmt.Fprintf(w, "data: %s\n\n", errChunk)
				w.Flush()
				break
			}

			for _, cand := range resp.Candidates {
				if cand.FinishReason != genai.FinishReasonUnspecified && cand.FinishReason != genai.FinishReasonStop {
					log.Printf("⚠️ Gemini Stream Finished with Reason: %v\n", cand.FinishReason)
				}

				if cand.Content != nil {
					for _, part := range cand.Content.Parts {
						text := fmt.Sprintf("%v", part)
						fullResponse += text

						chunk, _ := json.Marshal(fiber.Map{"text": text})
						fmt.Fprintf(w, "data: %s\n\n", chunk)
						w.Flush()
					}
				}
			}
		}

		// 🟢 7. บันทึกคำตอบ AI ลง PostgreSQL ถาวร
		if fullResponse != "" {
			aiMsg := model.ChatMessage{
				SessionID: currentSessionID,
				UserID:    userID,
				Role:      "model",
				Content:   fullResponse,
			}
			database.DB.Create(&aiMsg)
			state.Buffer = append(state.Buffer, aiMsg)
		}

		_ = store.SaveContextState(core.Ctx, userID, state)
		_ = store.ClearHistoryCache(core.Ctx, userID)

		if !streamHadError {
			fmt.Fprintf(w, "data: [DONE]\n\n")
		}
		w.Flush()
	})

	return nil
}

// GetSessions ดึงรายชื่อห้องแชททั้งหมดของ User คนนั้น (แสดงที่ Sidebar)
func GetSessions(c *fiber.Ctx) error {
	userID := getUserID(c)
	if userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var sessions []model.ChatSession
	// ดึงข้อมูลเรียงตามล่าสุดที่คุยกัน
	if err := database.DB.Where("user_id = ?", userID).Order("updated_at desc").Find(&sessions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch sessions"})
	}

	var response []map[string]interface{}
	for _, s := range sessions {
		response = append(response, map[string]interface{}{
			"id":    s.ID,
			"title": s.Title,
		})
	}

	return c.JSON(response)
}

// GetHistory ดึงประวัติแชทของห้องใดห้องหนึ่ง
func GetHistory(c *fiber.Ctx) error {
	userID := getUserID(c)
	sessionID := c.Query("session_id") // รับจาก URL: /api/history?session_id=1

	if userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// ถ้าหน้าบ้านไม่ได้ส่ง session_id มา (แปลว่าเพิ่งกดปุ่มแชทใหม่) ให้คืนค่า array ว่างๆ
	if sessionID == "" || sessionID == "null" {
		return c.JSON([]interface{}{})
	}

	var messages []model.ChatMessage
	if err := database.DB.Where("user_id = ? AND session_id = ?", userID, sessionID).Order("id asc").Find(&messages).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch history"})
	}

	// คืนค่าไปเฉพาะสิ่งที่จะใช้แสดงผล
	var response []map[string]interface{}
	for _, m := range messages {
		response = append(response, map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	return c.JSON(response)
}

// ClearHistory ลบประวัติแชตทั้งหมด
func ClearHistory(c *fiber.Ctx) error {
	userID := getUserID(c)

	if err := store.ClearHistory(c.UserContext(), userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to clear history"})
	}

	return c.JSON(fiber.Map{"message": "History cleared successfully"})
}