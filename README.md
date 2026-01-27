# Play-Arenas

[![Go](https://img.shields.io/badge/Go-1.25-blue)](https://golang.org) [![Gin](https://img.shields.io/badge/Gin-Framework-blueviolet)](https://github.com/gin-gonic/gin) [![GORM](https://img.shields.io/badge/GORM-gorm-success)](https://gorm.io) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13-blue?logo=postgresql)](https://www.postgresql.org) [![Kafka](https://img.shields.io/badge/Kafka-Apache-red?logo=apachekafka)](https://kafka.apache.org) [![Docker](https://img.shields.io/badge/Docker-Compose-blue?logo=docker)](https://www.docker.com) [![JWT](https://img.shields.io/badge/JWT-json--web--tokens-yellowgreen)](https://jwt.io)

Короткое описание
- ProPlay-Arenas — микросервисная система для поиска и бронирования спортивных площадок. Включает Gateway и сервисы: user, venue, reservation, payment. Использует Go, Gin, GORM, PostgreSQL, Kafka и Docker.



Ключевые функции
-Забронировать поле
-Удобность бронирования
-Регистрация под покупателя и владельца
-Расписание полей
-Просмотр доступных слотов на заданную дату

```
             HTTP      ┌───────────────────┐
┌────────┐ ──────────> │  Gateway Service  │
│ Client │             └─────────┬─────────┘
└────────┘                       │
           ┌─────────────────────┼─────────────────────┐
           │                     │                     │
           v                     v                     v
    ┌──────────────┐      ┌──────────────┐      ┌──────────────┐
    │    Venue     │◄─────│   Booking    │      │    User      │
    │   Service    │ HTTP │   Service    │      │   Service    │
    └──────┬───────┘      └──────┬───────┘      └──────┬───────┘
           │ (check              │                     │
           │  schedule)     PostgreSQL            PostgreSQL
      PostgreSQL                 │
                                 │ produces
                          Kafka Topics
                     ┌─────────────────────┐
                     │ booking.created     │───────┐
                     │ booking.cancelled   │       │ consumes
                     └─────────────────────┘       │
                                                   v
                                            ┌──────────────┐
                                            │   Payment    │
                                            │   Service    │
                                            └──────┬───────┘
                                                   │
                                              PostgreSQL

```

                                              
Варианты запуска
```bash
# Быстрый запуск всех сервисов (Docker Compose):
docker-compose up --build


# Запуск в фоне:
docker-compose up -d --build


# Остановить и удалить контейнеры:
docker-compose down


#Локальный запуск отдельного сервиса (пример для сервиса user):
cd user-service
go run ./cmd/app
```

Участники:
- Цуруев Имран - https://github.com/tsuruevimran17
- Байсангур Идигов - https://github.com/Idigov
- Шадид Яскиев - https://github.com/DjMariarty
- Хамзат Гериев - https://github.com/namexamz


