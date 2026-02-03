package models

import (
	"time"

	"gorm.io/gorm"
)

type PaymentStatus string
type PaymentMethod string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
	PaymentStatusFailed    PaymentStatus = "failed"

	MethodCard PaymentMethod = "card"
	MethodCash PaymentMethod = "cash"
)

var AllowedPaymentMethods = map[PaymentMethod]struct{}{
	MethodCard: {},
	MethodCash: {},
}

func IsValidPaymentMethod(method PaymentMethod) bool {
	_, ok := AllowedPaymentMethods[method]
	return ok
}

type Payment struct {
	gorm.Model
	BookingID      uint          `gorm:"index" json:"booking_id"`
	UserID         uint          `gorm:"index" json:"user_id"`
	Amount         int64         `gorm:"column:amount" json:"amount"`
	Currency       string        `gorm:"column:currency" json:"currency"`
	Method         PaymentMethod `gorm:"column:method" json:"method"`
	Status         PaymentStatus `gorm:"column:status" json:"status"`
	RefundedAmount int64         `gorm:"column:refunded_amount;default:0" json:"refunded_amount"`
	PaidAt         *time.Time    `gorm:"column:paid_at" json:"paid_at"`
	RefundedAt     *time.Time    `gorm:"column:refunded_at" json:"refunded_at"`
}
