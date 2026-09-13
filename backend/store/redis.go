package store

import (
	"context"
	"encoding/json"
	"fmt"
	//"time"

	"yuuna/model"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

type ChatContextState struct {
	Summary string              `json:"summary"`
	Buffer  []model.ChatMessage `json:"buffer"`
}

func InitRedis(addr, password string, db int) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return RedisClient.Ping(context.Background()).Err()
}

func getSessionKey(userID uint) string {
	return fmt.Sprintf("chat:user:%d:session", userID)
}

func getHistoryKey(userID uint) string {
	return fmt.Sprintf("chat:user:%d:history", userID)
}

// GetContextState ดึงทั้ง Summary และ Buffer จาก Redis
func GetContextState(ctx context.Context, userID uint) (*ChatContextState, error) {
	key := getSessionKey(userID)
	val, err := RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return &ChatContextState{Summary: "", Buffer: []model.ChatMessage{}}, nil
	} else if err != nil {
		return nil, err
	}

	var state ChatContextState
	err = json.Unmarshal([]byte(val), &state)
	return &state, err
}

// SaveContextState บันทึกทั้ง Summary และ Buffer ลง Redis (TTL 24 ชั่วโมง)
func SaveContextState(ctx context.Context, userID uint, state *ChatContextState) error {
	key := getSessionKey(userID)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return RedisClient.Set(ctx, key, data, 0).Err() // 👈 เปลี่ยนเป็น 0
}

// ClearContextState ลบเฉพาะ Context State
func ClearContextState(ctx context.Context, userID uint) error {
	return RedisClient.Del(ctx, getSessionKey(userID)).Err()
}

// GetHistory ดึงประวัติแชตจาก Cache
func GetHistory(ctx context.Context, userID uint) ([]model.ChatMessage, error) {
	key := getHistoryKey(userID)
	val, err := RedisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var history []model.ChatMessage
	err = json.Unmarshal([]byte(val), &history)
	return history, err
}

// SaveHistory บันทึกประวัติแชตลง Cache
func SaveHistory(ctx context.Context, userID uint, messages []model.ChatMessage) error {
	key := getHistoryKey(userID)
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	return RedisClient.Set(ctx, key, data, 0).Err() // 👈 เปลี่ยนเป็น 0
}

// ClearHistoryCache ลบเฉพาะ History Cache
func ClearHistoryCache(ctx context.Context, userID uint) error {
	return RedisClient.Del(ctx, getHistoryKey(userID)).Err()
}

// ClearHistory ลบทั้ง History Cache และ Context State
func ClearHistory(ctx context.Context, userID uint) error {
	return RedisClient.Del(ctx, getHistoryKey(userID), getSessionKey(userID)).Err()
}