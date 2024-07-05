package config

import (
	"log"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/joho/godotenv"
)

var DB *gorm.DB

func ConnectSQL() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	database, err := gorm.Open("sqlite3", "telegrambot.db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	DB = database
}
