package transport

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"payment-service/internal/dto"
	"payment-service/internal/models"
	"payment-service/internal/repository"
	"payment-service/internal/services"
)

type PaymentHandler struct {
	paymentService services.PaymentService
	refundService  services.RefundService
	logger         *slog.Logger
}

func NewPaymentHandler(paymentService services.PaymentService, refundService services.RefundService, logger *slog.Logger) *PaymentHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentHandler{
		paymentService: paymentService,
		refundService:  refundService,
		logger:         logger,
	}
}

func (h *PaymentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	payments := rg.Group("/payments")
	{
		payments.POST("", h.CreatePayment)
		payments.GET("", h.GetPaymentsHistory)
		payments.GET("/:id", h.GetPaymentByID)
		payments.POST("/:id/refund", h.CreateRefund)
	}

	bookings := rg.Group("/bookings")
	{
		bookings.GET("/:id/payment", h.GetPaymentByBookingID)
	}
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req dto.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "ОШИБКА_ВАЛИДАЦИИ", "400", err.Error())
		return
	}

	payment, err := h.paymentService.CreatePayment(&req)
	if err != nil {
		if isClientError(err) {
			writeError(c, http.StatusBadRequest, "ОШИБКА_ПЛАТЕЖА", "400", err.Error())
			return
		}
		h.logger.Error("не удалось создать платеж", "error", err)
		writeError(c, http.StatusInternalServerError, "ВНУТРЕННЯЯ_ОШИБКА", "500", "внутренняя ошибка сервера")
		return
	}

	c.JSON(http.StatusCreated, paymentToResponse(payment))
}

func (h *PaymentHandler) GetPaymentByID(c *gin.Context) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "НЕКОРРЕКТНЫЙ_ID", "400", "некорректный id платежа")
		return
	}

	payment, err := h.paymentService.GetPaymentByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(c, http.StatusNotFound, "ОШИБКА", "404", "платеж не найден")
			return
		}
		h.logger.Error("ошибка получения платежа по id", "error", err, "payment_id", id)
		writeError(c, http.StatusInternalServerError, "ВНУТРЕННЯЯ_ОШИБКА", "500", "внутренняя ошибка сервера")
		return
	}

	c.JSON(http.StatusOK, paymentToResponse(payment))
}

func (h *PaymentHandler) GetPaymentsHistory(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-Id")
	if userIDStr == "" {
		userIDStr = c.Query("user_id")
	}
	if userIDStr == "" {
		writeError(c, http.StatusBadRequest, "НЕТ_ПАРАМЕТРА", "400", "отсутствует user_id")
		return
	}

	userID, err := parseUintID(userIDStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "НЕКОРРЕКТНЫЙ_ID", "400", "некорректный user_id")
		return
	}

	limit, offset := parseLimitOffset(c)
	payments, total, err := h.paymentService.GetPaymentsByUserID(userID, limit, offset)
	if err != nil {
		h.logger.Error("ошибка получения истории платежей", "error", err, "user_id", userID)
		writeError(c, http.StatusInternalServerError, "ВНУТРЕННЯЯ_ОШИБКА", "500", "внутренняя ошибка сервера")
		return
	}

	resp := dto.PaymentHistoryResponse{
		Payments: make([]dto.PaymentResponse, len(payments)),
		Total:    total,
		Count:    len(payments),
	}
	for i := range payments {
		resp.Payments[i] = *paymentToResponse(&payments[i])
	}

	c.JSON(http.StatusOK, resp)
}

func (h *PaymentHandler) CreateRefund(c *gin.Context) {
	paymentID, err := parseUintID(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "НЕКОРРЕКТНЫЙ_ID", "400", "некорректный id платежа")
		return
	}

	var req dto.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "ОШИБКА_ВАЛИДАЦИИ", "400", err.Error())
		return
	}

	refund, err := h.refundService.CreateRefund(paymentID, &req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(c, http.StatusNotFound, "ОШИБКА", "404", "платеж не найден")
			return
		}
		if isClientError(err) {
			writeError(c, http.StatusBadRequest, "ОШИБКА_ВОЗВРАТА", "400", err.Error())
			return
		}
		h.logger.Error("не удалось создать возврат", "error", err, "payment_id", paymentID)
		writeError(c, http.StatusInternalServerError, "ВНУТРЕННЯЯ_ОШИБКА", "500", "внутренняя ошибка сервера")
		return
	}

	c.JSON(http.StatusCreated, refundToResponse(refund))
}

func (h *PaymentHandler) GetPaymentByBookingID(c *gin.Context) {
	bookingID, err := parseUintID(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "НЕКОРРЕКТНЫЙ_ID", "400", "некорректный id брони")
		return
	}

	payment, err := h.paymentService.GetPaymentByBookingID(bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(c, http.StatusNotFound, "ERROR", "404", "платеж не найден")
			return
		}
		h.logger.Error("ошибка получения платежа по booking_id", "error", err, "booking_id", bookingID)
		writeError(c, http.StatusInternalServerError, "ВНУТРЕННЯЯ_ОШИБКА", "500", "внутренняя ошибка сервера")
		return
	}

	c.JSON(http.StatusOK, paymentToResponse(payment))
}

func paymentToResponse(payment *models.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		ID:             payment.ID,
		BookingID:      payment.BookingID,
		UserID:         payment.UserID,
		Amount:         payment.Amount,
		Currency:       payment.Currency,
		Method:         payment.Method,
		Status:         payment.Status,
		RefundedAmount: payment.RefundedAmount,
		PaidAt:         payment.PaidAt,
		RefundedAt:     payment.RefundedAt,
		CreatedAt:      payment.CreatedAt,
		UpdatedAt:      payment.UpdatedAt,
	}
}

func refundToResponse(refund *models.Refund) *dto.RefundResponse {
	return &dto.RefundResponse{
		ID:        refund.ID,
		PaymentID: refund.PaymentID,
		Amount:    refund.Amount,
		Reason:    refund.Reason,
		Status:    refund.Status,
		CreatedAt: refund.CreatedAt,
		UpdatedAt: refund.UpdatedAt,
	}
}

func writeError(c *gin.Context, status int, errCode, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error:   errCode,
		Code:    code,
		Message: message,
	})
}

func parseUintID(raw string) (uint, error) {
	id64, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id64), nil
}

func parseLimitOffset(c *gin.Context) (int, int) {
	limit := 10
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	return limit, offset
}

func isClientError(err error) bool {
	return errors.Is(err, services.ErrEmptyRequest) ||
		errors.Is(err, services.ErrInvalidAmount) ||
		errors.Is(err, services.ErrInvalidMethod) ||
		errors.Is(err, services.ErrPaymentNotComplete) ||
		errors.Is(err, services.ErrRefundAmountExceed)
}
