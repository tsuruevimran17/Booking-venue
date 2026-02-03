package services

import (
	"log/slog"
	"time"

	"payment-service/internal/dto"
	"payment-service/internal/models"
	"payment-service/internal/repository"
)

type PaymentService interface {
	CreatePayment(req *dto.CreatePaymentRequest) (*models.Payment, error)
	CreatePendingPayment(req *dto.CreatePaymentRequest) (*models.Payment, error)
	GetPaymentByID(id uint) (*models.Payment, error)
	GetPaymentByBookingID(bookingID uint) (*models.Payment, error)
	GetPaymentsByUserID(userID uint, limit, offset int) ([]models.Payment, int64, error)
}

type PaymentServiceImpl struct {
	paymentRepo repository.PaymentRepository
	logger      *slog.Logger
}

func NewPaymentService(paymentRepo repository.PaymentRepository) PaymentService {
	return &PaymentServiceImpl{
		paymentRepo: paymentRepo,
		logger:      slog.Default(),
	}
}

func (s *PaymentServiceImpl) CreatePayment(req *dto.CreatePaymentRequest) (*models.Payment, error) {
	return s.createPayment(req, models.PaymentStatusCompleted, true)
}

func (s *PaymentServiceImpl) CreatePendingPayment(req *dto.CreatePaymentRequest) (*models.Payment, error) {
	return s.createPayment(req, models.PaymentStatusPending, false)
}

func (s *PaymentServiceImpl) GetPaymentByID(id uint) (*models.Payment, error) {
	payment, err := s.paymentRepo.GetPaymentByID(id)
	if err != nil {
		s.logger.Error("ошибка получения платежа по id", "payment_id", id, "error", err)
		return nil, err
	}
	return payment, nil
}

func (s *PaymentServiceImpl) GetPaymentByBookingID(bookingID uint) (*models.Payment, error) {
	payment, err := s.paymentRepo.GetPaymentByBookingID(bookingID)
	if err != nil {
		s.logger.Error("ошибка получения платежа по booking_id", "booking_id", bookingID, "error", err)
		return nil, err
	}
	return payment, nil
}

func (s *PaymentServiceImpl) GetPaymentsByUserID(userID uint, limit, offset int) ([]models.Payment, int64, error) {
	payments, total, err := s.paymentRepo.GetPaymentsByUserID(userID, limit, offset)
	if err != nil {
		s.logger.Error("ошибка получения платежей пользователя", "user_id", userID, "error", err)
		return nil, 0, err
	}
	return payments, total, nil
}

func (s *PaymentServiceImpl) createPayment(req *dto.CreatePaymentRequest, status models.PaymentStatus, setPaidAt bool) (*models.Payment, error) {
	if req == nil {
		s.logger.Error("пустой запрос на создание платежа")
		return nil, ErrEmptyRequest
	}
	if req.Amount <= 0 {
		s.logger.Error("некорректная сумма платежа", "amount", req.Amount)
		return nil, ErrInvalidAmount
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}
	if !models.IsValidPaymentMethod(req.Method) {
		s.logger.Error("недопустимый метод оплаты", "method", req.Method)
		return nil, ErrInvalidMethod
	}

	payment := &models.Payment{
		BookingID: req.BookingID,
		UserID:    req.UserID,
		Amount:    req.Amount,
		Currency:  req.Currency,
		Method:    req.Method,
		Status:    status,
	}
	if setPaidAt {
		now := time.Now()
		payment.PaidAt = &now
	}

	if err := s.paymentRepo.CreatePayment(payment); err != nil {
		s.logger.Error("ошибка сохранения платежа", "error", err)
		return nil, err
	}

	s.logger.Info("платеж создан", "payment_id", payment.ID)
	return payment, nil
}
