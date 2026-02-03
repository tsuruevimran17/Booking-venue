package main

import (
	"fmt"
	"log/slog"
	"os"

	"venue-service/internal/config"
	"venue-service/internal/repository"
	"venue-service/internal/services"
	"venue-service/internal/transport"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("ошибка загрузки переменных окружения", "error", err)
	}

	logger := config.InitLogger()

	db, err := config.ConnectDB()
	if err != nil {
		logger.Error("Ошибка подключения к БД", "layer", "config", "error", err)
		os.Exit(1)
	}

	venueRepo := repository.NewVenueRepository(db, logger)
	venueService := services.NewVenueService(venueRepo, logger)
	r := gin.Default()

	// Отключаем доверие прокси для локальной разработки
	r.SetTrustedProxies(nil)

	transport.RegisterRoutes(r, logger, venueService)

	if err := r.Run(fmt.Sprintf(":%s", config.GetEnv("PORT", "8080"))); err != nil {
		slog.Error("ошибка запуска сервера", "error", err)
		os.Exit(1)
	}
}
