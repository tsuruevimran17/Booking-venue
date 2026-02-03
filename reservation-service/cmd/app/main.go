package main

import (
	"log/slog"
	"os"

	"reservation/internal/config"
	"reservation/internal/kafka"
	"reservation/internal/models"
	"reservation/internal/repository"
	"reservation/internal/service"
	"reservation/internal/transport"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Загружаем .env только для локальной разработки
	// В Docker все переменные передаются через docker-compose.yaml
	if err := godotenv.Load(); err != nil {
		slog.Info(".env file not found, using system environment variables")
	}

	db := config.SetUpDatabaseConnection()

	if err := db.AutoMigrate(&models.ReservationDetails{}); err != nil {
		slog.Error("ошибка миграции базы данных", "error", err)
		os.Exit(1)
	}

	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		slog.Error("KAFKA_BROKERS не задан в переменных окружения")
		os.Exit(1)
	}

	producer := kafka.NewProducer([]string{kafkaBrokers})
	defer func() {
		if err := producer.Close(); err != nil {
			slog.Error("ошибка закрытия Kafka продюсера", "error", err)
		}
	}()

	bookingRepo := repository.NewBookingRepo(db)
	venueServiceURL := os.Getenv("VENUE_SERVICE_URL")
	if venueServiceURL == "" {
		slog.Error("VENUE_SERVICE_URL не задан в переменных окружения")
		os.Exit(1)
	}
	bookingServ := service.NewBookingServ(bookingRepo, producer, venueServiceURL, db)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET не задан в переменных окружения")
		os.Exit(1)
	}

	r := gin.Default()

	transport.RegisterRoutes(r, bookingServ, jwtSecret)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	slog.Info("сервер запущен", "port", port)
	if err := r.Run(":" + port); err != nil {
		slog.Error("не удалось запустить сервер", "error", err)
		os.Exit(1)
	}
}
