package dto

import (
	"time"

	"payment-service/internal/models"
)

type CreatePaymentRequest struct {
	BookingID uint                 `json:"booking_id" binding:"required"`
	UserID    uint                 `json:"user_id" binding:"required"`
	Amount    int64                `json:"amount" binding:"required,gt=0"`
	Currency  string               `json:"currency" binding:"omitempty,oneof=RUB"`
	Method    models.PaymentMethod `json:"method" binding:"required"`
}

type PaymentResponse struct {
	ID             uint                 `json:"id"`
	BookingID      uint                 `json:"booking_id"`
	UserID         uint                 `json:"user_id"`
	Amount         int64                `json:"amount"`
	Currency       string               `json:"currency"`
	Method         models.PaymentMethod `json:"method"`
	Status         models.PaymentStatus `json:"status"`
	RefundedAmount int64                `json:"refunded_amount"`
	PaidAt         *time.Time           `json:"paid_at"`
	RefundedAt     *time.Time           `json:"refunded_at"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type PaymentHistoryResponse struct {
	Payments []PaymentResponse `json:"payments"`
	Total    int64             `json:"total"`
	Count    int               `json:"count"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
