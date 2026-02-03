package services

import (
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"payment-service/internal/dto"
	"payment-service/internal/models"
	"payment-service/internal/repository"
)

type RefundService interface {
	CreateRefund(paymentID uint, req *dto.RefundRequest) (*models.Refund, error)
	GetRefundByID(id uint) (*models.Refund, error)
	GetRefundsByPaymentID(paymentID uint) ([]models.Refund, error)
}

type RefundServiceImpl struct {
	refundRepo  repository.RefundRepository
	paymentRepo repository.PaymentRepository
	logger      *slog.Logger
	db          *gorm.DB
}

func NewRefundService(refundRepo repository.RefundRepository, paymentRepo repository.PaymentRepository, db *gorm.DB) RefundService {
	return &RefundServiceImpl{
		refundRepo:  refundRepo,
		paymentRepo: paymentRepo,
		logger:      slog.Default(),
		db:          db,
	}
}

func (s *RefundServiceImpl) CreateRefund(paymentID uint, req *dto.RefundRequest) (*models.Refund, error) {
	if req == nil {
		s.logger.Error("пустой запрос на возврат")
		return nil, ErrEmptyRequest
	}

	var createdRefund *models.Refund
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		paymentRepo := repository.NewPaymentRepository(tx)
		refundRepo := repository.NewRefundRepository(tx)

		var payment models.Payment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, paymentID).Error; err != nil {
			s.logger.Error("ошибка получения платежа для возврата", "payment_id", paymentID, "error", err)
			return err
		}

		if payment.Status != models.PaymentStatusCompleted {
			s.logger.Error("платеж не завершен", "payment_id", paymentID, "status", payment.Status)
			return ErrPaymentNotComplete
		}

		available := payment.Amount - payment.RefundedAmount
		if req.Amount > available {
			s.logger.Error("сумма возврата превышает доступную", "payment_id", paymentID, "amount", req.Amount, "available", available)
			return ErrRefundAmountExceed
		}

		refund := &models.Refund{
			PaymentID: paymentID,
			Amount:    req.Amount,
			Reason:    req.Reason,
			Status:    models.RefundStatusCompleted,
		}

		if err := refundRepo.CreateRefund(refund); err != nil {
			s.logger.Error("не удалось создать возврат", "error", err)
			return err
		}

		payment.RefundedAmount += req.Amount
		if payment.RefundedAmount >= payment.Amount {
			payment.Status = models.PaymentStatusRefunded
			now := time.Now()
			payment.RefundedAt = &now
		}

		if err := paymentRepo.UpdatePayment(payment); err != nil {
			s.logger.Error("ошибка обновления платежа после возврата", "payment_id", payment.ID, "error", err)
			return err
		}

		createdRefund = refund
		return nil
	}); err != nil {
		return nil, err
	}

	s.logger.Info("возврат создан", "refund_id", createdRefund.ID, "payment_id", paymentID)
	return createdRefund, nil
}

func (s *RefundServiceImpl) GetRefundByID(id uint) (*models.Refund, error) {
	refund, err := s.refundRepo.GetRefundByID(id)
	if err != nil {
		s.logger.Error("ошибка получения возврата по id", "refund_id", id, "error", err)
		return nil, err
	}
	return refund, nil
}

func (s *RefundServiceImpl) GetRefundsByPaymentID(paymentID uint) ([]models.Refund, error) {
	refunds, err := s.refundRepo.GetRefundsByPaymentID(paymentID)
	if err != nil {
		s.logger.Error("ошибка получения возвратов по payment_id", "payment_id", paymentID, "error", err)
		return nil, err
	}
	return refunds, nil
}
