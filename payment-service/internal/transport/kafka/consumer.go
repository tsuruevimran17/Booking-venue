package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"

	kafkago "github.com/segmentio/kafka-go"

	"payment-service/internal/config"
	"payment-service/internal/dto"
	"payment-service/internal/models"
	"payment-service/internal/repository"
	"payment-service/internal/services"
)

type Consumer struct {
	paymentService services.PaymentService
	refundService  services.RefundService
	logger         *slog.Logger
	brokers        []string
	groupID        string
	createdTopic   string
	cancelledTopic string
}

type BookingCreatedEvent struct {
	BookingID uint    `json:"booking_id"`
	ClientID  uint    `json:"client_id"`
	Price     float64 `json:"price_cents"`
}

type BookingCancelledEvent struct {
	BookingID uint `json:"booking_id"`
}

func NewConsumerFromEnv(paymentService services.PaymentService, refundService services.RefundService, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}

	brokers := splitBrokers(config.GetEnv("KAFKA_BROKERS", ""))
	return &Consumer{
		paymentService: paymentService,
		refundService:  refundService,
		logger:         logger,
		brokers:        brokers,
		groupID:        config.GetEnv("KAFKA_GROUP_ID", "payment-service"),
		createdTopic:   config.GetEnv("KAFKA_TOPIC_BOOKING_CREATED", "booking.created"),
		cancelledTopic: config.GetEnv("KAFKA_TOPIC_BOOKING_CANCELLED", "booking.cancelled"),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	if len(c.brokers) == 0 {
		c.logger.Warn("Kafka brokers not configured, consumer disabled")
		return
	}

	go c.consumeBookingCreated(ctx)
	go c.consumeBookingCancelled(ctx)
}

func (c *Consumer) consumeBookingCreated(ctx context.Context) {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: c.brokers,
		GroupID: c.groupID,
		Topic:   c.createdTopic,
	})
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("ошибка чтения сообщения booking.created", "error", err)
			continue
		}

		var event BookingCreatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error("ошибка парсинга booking.created", "error", err)
			continue
		}

		if event.BookingID == 0 || event.ClientID == 0 {
			c.logger.Error("некорректные данные booking.created", "booking_id", event.BookingID, "client_id", event.ClientID)
			continue
		}

		amount := int64(math.Round(event.Price))
		if amount <= 0 {
			c.logger.Error("некорректная сумма в booking.created", "price_cents", event.Price)
			continue
		}

		existing, err := c.paymentService.GetPaymentByBookingID(event.BookingID)
		if err == nil && existing != nil {
			continue
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			c.logger.Error("ошибка проверки существующего платежа", "error", err, "booking_id", event.BookingID)
			continue
		}

		req := dto.CreatePaymentRequest{
			BookingID: event.BookingID,
			UserID:    event.ClientID,
			Amount:    amount,
			Currency:  "RUB",
			Method:    models.MethodCard,
		}

		if _, err := c.paymentService.CreatePendingPayment(&req); err != nil {
			c.logger.Error("ошибка создания pending платежа из booking.created", "error", err, "booking_id", event.BookingID)
			continue
		}
	}
}

func (c *Consumer) consumeBookingCancelled(ctx context.Context) {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers: c.brokers,
		GroupID: c.groupID,
		Topic:   c.cancelledTopic,
	})
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("ошибка чтения сообщения booking.cancelled", "error", err)
			continue
		}

		var event BookingCancelledEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error("ошибка парсинга booking.cancelled", "error", err)
			continue
		}

		if event.BookingID == 0 {
			c.logger.Error("некорректные данные booking.cancelled", "booking_id", event.BookingID)
			continue
		}

		payment, err := c.paymentService.GetPaymentByBookingID(event.BookingID)
		if err != nil {
			c.logger.Error("ошибка получения платежа по booking_id для возврата", "error", err, "booking_id", event.BookingID)
			continue
		}

		if payment.Status != models.PaymentStatusCompleted {
			continue
		}

		remaining := payment.Amount - payment.RefundedAmount
		if remaining <= 0 {
			continue
		}

		if _, err := c.refundService.CreateRefund(payment.ID, &dto.RefundRequest{
			Amount: remaining,
			Reason: "отмена бронирования",
		}); err != nil {
			c.logger.Error("ошибка создания возврата по booking.cancelled", "error", err, "payment_id", payment.ID)
			continue
		}
	}
}

func splitBrokers(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
