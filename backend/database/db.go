package database

import (
	"log"
	"yuuna/model" // เปลี่ยนเป็นชื่อ module ใน go.mod ของคุณ

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB(dsn string) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("❌ Failed to connect database:", err)
	}

	// Auto Migrate ตาราง ChatMessage
	err = DB.AutoMigrate(
		&model.User{},
		&model.ChatSession{},
		&model.ChatMessage{},
		
	)
	if err != nil {
		log.Fatal("❌ Failed to migrate database:", err)
	}

	log.Println("✅ Database connected & migrated successfully")
}