# ProPlay-Arenas

[![Go](https://img.shields.io/badge/Go-1.25-blue)](https://golang.org) [![Gin](https://img.shields.io/badge/Gin-Framework-blueviolet)](https://github.com/gin-gonic/gin) [![GORM](https://img.shields.io/badge/GORM-gorm-success)](https://gorm.io) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13-blue?logo=postgresql)](https://www.postgresql.org) [![Kafka](https://img.shields.io/badge/Kafka-Apache-red?logo=apachekafka)](https://kafka.apache.org) [![Docker](https://img.shields.io/badge/Docker-Compose-blue?logo=docker)](https://www.docker.com) [![JWT](https://img.shields.io/badge/JWT-json--web--tokens-yellowgreen)](https://jwt.io)

Короткое описание
- ProPlay-Arenas — микросервисная система для поиска и бронирования спортивных площадок. Включает Gateway и сервисы: user, venue, reservation, payment. Использует Go, Gin, GORM, PostgreSQL, Kafka и Docker.

Варианты запуска
- Быстрый запуск всех сервисов (Docker Compose):

```bash
docker-compose up --build
```

- Запуск в фоне:

```bash
docker-compose up -d --build
```

- Остановить и удалить контейнеры:

```bash
docker-compose down
```

- Локальный запуск отдельного сервиса (пример для сервиса user):

```bash
cd user-service
go run ./cmd/app
```

Переменные окружения
- По умолчанию используйте файл `.env` в корне проекта. Минимально требуется:

```
JWT_SECRET=your-super-secret-jwt-key
```
