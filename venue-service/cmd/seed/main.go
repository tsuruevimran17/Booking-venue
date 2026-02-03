package main

import (
	"flag"
	"log/slog"
	"os"

	"venue-service/internal/config"
	"venue-service/internal/seed"

	"github.com/joho/godotenv"
)

func main() {
	forceFlag := flag.Bool("force", false, "Принудительно перезаписать данные (удалить существующие и создать новые)")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		slog.Warn("ошибка загрузки переменных окружения", "error", err)
	}

	logger := config.InitLogger()

	db, err := config.ConnectDB()
	if err != nil {
		logger.Error("Ошибка подключения к БД", "layer", "config", "error", err)
		os.Exit(1)
	}

	if *forceFlag {
		logger.Info("Запуск сидов с принудительным перезаписыванием")
		if err := seed.SeedVenuesForce(db, logger); err != nil {
			logger.Error("Ошибка выполнения сидов", "error", err)
			os.Exit(1)
		}
		logger.Info("Сиды успешно выполнены")
		return
	}

	logger.Info("Запуск сидов")
	if err := seed.SeedVenues(db, logger); err != nil {
		logger.Error("Ошибка выполнения сидов", "error", err)
		os.Exit(1)
	}
	logger.Info("Сиды успешно выполнены")
}
