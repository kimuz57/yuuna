package core

import (
	"context"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var GeminiClient *genai.Client
var Ctx = context.Background()

func InitGemini(apiKey string) {
	var err error
	GeminiClient, err = genai.NewClient(Ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal("❌ Failed to create Gemini client:", err)
	}
	log.Println("✅ Gemini API Ready")
}