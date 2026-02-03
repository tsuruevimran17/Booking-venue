package config

import (
	"fmt"
	"log/slog"
	"os"
	"user-service/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDatabase() *gorm.DB {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	if dbHost == "" || dbUser == "" || dbName == "" || dbPort == "" {
		slog.Error("one or more required environment variables are missing: DB_HOST, DB_USER, DB_NAME, DB_PORT")
		os.Exit(1)
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		dbHost,
		dbUser,
		dbPassword,
		dbName,
		dbPort,
		dbSSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}

	// Автоматическая миграция моделей
	err = db.AutoMigrate(&models.User{})
	if err != nil {
		slog.Error("failed to auto-migrate models", "error", err)
		os.Exit(1)
	}

	return db

}
