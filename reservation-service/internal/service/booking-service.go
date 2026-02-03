package service

import (
	"context"
	"fmt"
	"log/slog"
	"reservation/internal/dto"
	"reservation/internal/errors"
	"reservation/internal/kafka"
	"reservation/internal/models"
	"reservation/internal/repository"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BookingService interface {
	GetUserReservations(userID uint) ([]models.Reservation, error)
	GetVenueBookings(venueID uint, claims *models.Claims) ([]models.ReservationDetails, error)
	GetVenueAvailability(venueID uint, date time.Time) ([]dto.AvailableSlot, error)
	CreateReservation(reservation *dto.ReservationCreate, claims *models.Claims) (*models.ReservationDetails, error)
	ReservationCancel(id uint, reason string) (*models.ReservationDetails, error)
	GetByID(id uint) (*models.ReservationDetails, error)
	ReservationUpdate(id uint, reservation *dto.ReservationUpdate) (*models.ReservationDetails, error)
}

type bookingService struct {
	repo     repository.BookingRepo
	producer kafka.Producer
	client   *resty.Client
	venueURL string
	db       *gorm.DB
}

func NewBookingServ(repo repository.BookingRepo, producer kafka.Producer, venueURL string, db *gorm.DB) BookingService {
	return &bookingService{repo: repo, producer: producer, client: resty.New(), venueURL: strings.TrimRight(venueURL, "/"), db: db}
}

func (r *bookingService) GetUserReservations(userID uint) ([]models.Reservation, error) {
	reservations, err := r.repo.GetUserReservations(userID)

	if err != nil {
		return nil, err
	}

	return reservations, nil
}

func (r *bookingService) GetVenueBookings(venueID uint, claims *models.Claims) ([]models.ReservationDetails, error) {

	if claims == nil {
		return nil, errors.ErrForbidden
	}

	if claims.Role != models.RoleOwner && claims.Role != models.RoleAdmin {
		return nil, errors.ErrForbidden
	}

	venue, err := r.GetVenue(venueID)
	if err != nil {
		return nil, err
	}

	if claims.Role != models.RoleAdmin && venue.OwnerID != claims.UserID {
		return nil, errors.ErrNotOwner
	}

	bookings, err := r.repo.GetVenueBookings(venueID)

	if err != nil {
		return nil, err
	}

	return bookings, nil
}

func (r *bookingService) GetByID(id uint) (*models.ReservationDetails, error) {
	reservation, err := r.repo.GetByID(id)

	if err != nil {
		return nil, err
	}

	return reservation, nil
}

func (r *bookingService) CreateReservation(reservation *dto.ReservationCreate, claims *models.Claims) (*models.ReservationDetails, error) {

	if reservation.OwnerID <= 0 {
		return nil, errors.ErrOwnerID
	}

	if reservation.StartAt.IsZero() {
		return nil, errors.ErrStartAtEmpty
	}

	if reservation.EndAt.IsZero() {
		return nil, errors.ErrEndAtEmpty
	}

	if !reservation.StartAt.Before(reservation.EndAt) {
		return nil, errors.ErrStartAtAfterEndAt
	}

	if reservation.StartAt.Before(time.Now()) {
		return nil, errors.ErrStartAtInPast
	}

	if reservation.Status == "" {
		return nil, errors.ErrStatusEmpty
	}

	if claims.Role != models.RoleClient && claims.Role != models.RoleAdmin {
		return nil, errors.ErrInvalidRole
	}

	if err := r.ValidateReservation(reservation); err != nil {
		return nil, err
	}

	reservation.ClientID = claims.UserID

	venue, err := r.GetVenue(reservation.VenueID)
	if err != nil {
		return nil, err
	}

	newReservation := &models.ReservationDetails{
		ClientID: reservation.ClientID,
		VenueID:  reservation.VenueID,
		OwnerID:  reservation.OwnerID,
		StartAt:  reservation.StartAt,
		EndAt:    reservation.EndAt,
		Price:    venue.HourPrice * reservation.EndAt.Sub(reservation.StartAt).Hours(),
		Status:   models.Status(reservation.Status),
		Duration: reservation.EndAt.Sub(reservation.StartAt),
	}

	if newReservation.Duration < time.Hour {
		return nil, errors.ErrDuration
	}

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.checkBookingConflictsTx(tx, reservation.VenueID, reservation.StartAt, reservation.EndAt, nil); err != nil {
			return err
		}
		return tx.Create(newReservation).Error
	}); err != nil {
		return nil, err
	}

	evt := dto.BookingCreatedEvent{
		EventID:   uuid.NewString(),
		CreatedAt: time.Now(),
		BookingID: newReservation.ID,
		VenueID:   newReservation.VenueID,
		ClientID:  newReservation.ClientID,
		OwnerID:   newReservation.OwnerID,
		StartAt:   newReservation.StartAt,
		EndAt:     newReservation.EndAt,
		Price:     newReservation.Price,
		Status:    newReservation.Status,
	}

	if err := r.producer.PublishBookingCreated(context.Background(), evt); err != nil {
		slog.Error("ошибка отправки события в Kafka", "error", err)
		return nil, fmt.Errorf("бронь создана (id=%d), но не удалось отправить событие в Kafka: %w", newReservation.ID, err)
	}

	return newReservation, nil
}

func (r *bookingService) ReservationCancel(id uint, reason string) (*models.ReservationDetails, error) {
	reservation, err := r.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if reservation.Status == models.Cancelled || reservation.Status == models.Completed {
		return nil, errors.ErrCannotCancel
	}

	reservation.Status = models.Cancelled
	reservation.ReasonForCancel = reason

	if err := r.repo.Save(reservation); err != nil {
		return nil, err
	}

	evt := dto.BookingCancelledEvent{
		EventID:   uuid.NewString(),
		CreatedAt: time.Now(),
		BookingID: reservation.ID,
		Reason:    reason,
		Status:    reservation.Status,
	}

	if err := r.producer.PublishBookingCancelled(context.Background(), evt); err != nil {
		slog.Error("ошибка отправки события отмены в Kafka", "error", err)
	}

	return reservation, nil
}

func (r *bookingService) ReservationUpdate(id uint, reservation *dto.ReservationUpdate) (*models.ReservationDetails, error) {

	reserv, err := r.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if reservation.ClientID != nil && *reservation.ClientID <= 0 {
		return nil, errors.ErrClientID
	}

	if reservation.OwnerID != nil && *reservation.OwnerID <= 0 {
		return nil, errors.ErrOwnerID
	}

	if reservation.StartAt != nil && reservation.StartAt.IsZero() {
		return nil, errors.ErrStartAtEmpty
	}

	if reservation.EndAt != nil && reservation.EndAt.IsZero() {
		return nil, errors.ErrEndAtEmpty
	}

	// Определяем финальные значения для валидации (не мутируя reserv заранее)
	finalStartAt := reserv.StartAt
	if reservation.StartAt != nil {
		finalStartAt = *reservation.StartAt
	}

	finalEndAt := reserv.EndAt
	if reservation.EndAt != nil {
		finalEndAt = *reservation.EndAt
	}

	// Проверяем, что StartAt < EndAt для итогового диапазона
	if !finalStartAt.Before(finalEndAt) {
		return nil, errors.ErrStartAtAfterEndAt
	}

	// Проверяем, что finalStartAt не в прошлом (независимо от того, обновляется ли он)
	if finalStartAt.Before(time.Now()) {
		return nil, errors.ErrStartAtInPast
	}

	// Дополнительная валидация: проверка расписания площадки и конфликтов броней
	if err := r.ValidateReservationUpdate(id, reservation); err != nil {
		return nil, err
	}

	if reservation.Price != nil && *reservation.Price <= 0 {
		return nil, errors.ErrNegativePrice
	}

	if reserv.Status != models.Pending {
		return nil, errors.ErrOnlyPendingReservations
	}

	if reservation.VenueID != nil {
		reserv.VenueID = *reservation.VenueID
	}

	if reservation.ClientID != nil {
		reserv.ClientID = *reservation.ClientID
	}

	if reservation.OwnerID != nil {
		reserv.OwnerID = *reservation.OwnerID
	}

	if reservation.StartAt != nil {
		reserv.StartAt = *reservation.StartAt
	}

	if reservation.EndAt != nil {
		reserv.EndAt = *reservation.EndAt
	}

	if reservation.Price != nil {
		reserv.Price = *reservation.Price
	}

	if err := r.repo.Save(reserv); err != nil {
		return nil, err
	}

	reserv.Duration = reserv.EndAt.Sub(reserv.StartAt)

	return reserv, nil

}

func (r *bookingService) GetVenue(id uint) (*dto.ResponsVenueServ, error) {

	url := fmt.Sprintf("%s/venues/%d", r.venueURL, id)

	var venue dto.ResponsVenueServ

	resp, err := r.client.R().SetResult(&venue).Get(url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Сервер вернул ошибку: %d", resp.StatusCode())
	}

	return &venue, nil
}

func (r *bookingService) ValidateReservation(reservation *dto.ReservationCreate) error {
	// Проверка: бронь должна быть в пределах одного дня (расписание задаётся по дням)
	if reservation.StartAt.Year() != reservation.EndAt.Year() ||
		reservation.StartAt.YearDay() != reservation.EndAt.YearDay() {
		return fmt.Errorf("бронь должна начинаться и заканчиваться в один день")
	}

	// Получаем расписание площадки
	venueFull, err := r.getVenueSchedule(reservation.VenueID)
	if err != nil {
		return err
	}

	// Определяем день недели и соответствующее расписание
	var day dto.DayScheduleDTO
	switch reservation.StartAt.Weekday() {
	case time.Monday:
		day = venueFull.Weekdays.Weekdays.Monday
	case time.Tuesday:
		day = venueFull.Weekdays.Weekdays.Tuesday
	case time.Wednesday:
		day = venueFull.Weekdays.Weekdays.Wednesday
	case time.Thursday:
		day = venueFull.Weekdays.Weekdays.Thursday
	case time.Friday:
		day = venueFull.Weekdays.Weekdays.Friday
	case time.Saturday:
		day = venueFull.Weekdays.Weekdays.Saturday
	case time.Sunday:
		day = venueFull.Weekdays.Weekdays.Sunday
	}

	if err := r.checkScheduleMatch(day, reservation.StartAt, reservation.EndAt); err != nil {
		return err
	}

	if err := r.checkBookingConflictsTx(r.db, reservation.VenueID, reservation.StartAt, reservation.EndAt, nil); err != nil {
		return err
	}

	return nil

}

// getVenueSchedule загружает полное представление площадки с расписанием
func (r *bookingService) getVenueSchedule(venueID uint) (*dto.ResponsVenueServFull, error) {
	url := fmt.Sprintf("%s/venues/%d", r.venueURL, venueID)
	var venueFull dto.ResponsVenueServFull
	resp, err := r.client.R().SetResult(&venueFull).Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Сервер вернул ошибку: %d", resp.StatusCode())
	}
	return &venueFull, nil
}

// checkScheduleMatch проверяет, что бронь попадает в рабочее время дня и корректно парсит расписание
func (r *bookingService) checkScheduleMatch(day dto.DayScheduleDTO, startAt, endAt time.Time) error {
	if !day.Enabled {
		return fmt.Errorf("площадка не работает в выбранный день")
	}
	if day.StartTime == nil || day.EndTime == nil {
		return fmt.Errorf("в расписании площадки отсутствует время работы для выбранного дня")
	}

	tStart, err := time.Parse("15:04", *day.StartTime)
	if err != nil {
		return fmt.Errorf("неверный формат start_time в расписании площадки: %w", err)
	}
	tEnd, err := time.Parse("15:04", *day.EndTime)
	if err != nil {
		return fmt.Errorf("неверный формат end_time в расписании площадки: %w", err)
	}

	venueStart := time.Date(startAt.Year(), startAt.Month(), startAt.Day(), tStart.Hour(), tStart.Minute(), 0, 0, startAt.Location())
	venueEnd := time.Date(endAt.Year(), endAt.Month(), endAt.Day(), tEnd.Hour(), tEnd.Minute(), 0, 0, endAt.Location())

	if startAt.Before(venueStart) || endAt.After(venueEnd) {
		return fmt.Errorf("бронь должна быть в пределах рабочего времени площадки: с %s по %s", (*day.StartTime), (*day.EndTime))
	}

	return nil
}

// checkBookingConflicts проверяет наличие конфликтующих броней в БД.
// Если excludeID != nil, то брони с этим id будут исключены (полезно для обновления).
func (r *bookingService) checkBookingConflictsTx(db *gorm.DB, venueID uint, startAt, endAt time.Time, excludeID *uint) error {
	var count int64
	q := db.Model(&models.Reservation{})
	if excludeID != nil {
		q = q.Where("venue_id = ? AND id <> ? AND ((start_at < ? AND end_at > ?) OR (start_at < ? AND end_at > ?) OR (start_at >= ? AND end_at <= ?))",
			venueID, *excludeID, endAt, endAt, startAt, startAt, startAt, endAt)
	} else {
		q = q.Where("venue_id = ? AND ((start_at < ? AND end_at > ?) OR (start_at < ? AND end_at > ?) OR (start_at >= ? AND end_at <= ?))",
			venueID, endAt, endAt, startAt, startAt, startAt, endAt)
	}
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("в выбранный период уже есть бронирования на эту площадку")
	}
	return nil
}

// ValidateReservationUpdate выполняет валидацию аналогичную ValidateReservation,
// но для DTO обновления брони. Принимает id существующей брони и dto.ReservationUpdate.
func (r *bookingService) ValidateReservationUpdate(id uint, reservation *dto.ReservationUpdate) error {
	// Получаем текущую бронь
	current, err := r.repo.GetByID(id)
	if err != nil {
		return err
	}

	// Определяем итоговые значения для проверки
	venueID := current.VenueID
	if reservation.VenueID != nil {
		venueID = *reservation.VenueID
	}

	finalStartAt := current.StartAt
	if reservation.StartAt != nil {
		finalStartAt = *reservation.StartAt
	}
	finalEndAt := current.EndAt
	if reservation.EndAt != nil {
		finalEndAt = *reservation.EndAt
	}

	// Бронь должна быть в пределах одного дня
	if finalStartAt.Year() != finalEndAt.Year() || finalStartAt.YearDay() != finalEndAt.YearDay() {
		return fmt.Errorf("бронь должна начинаться и заканчиваться в один день")
	}

	// Получаем расписание площадки
	venueFull, err := r.getVenueSchedule(venueID)
	if err != nil {
		return err
	}

	// Определяем день недели
	var day dto.DayScheduleDTO
	switch finalStartAt.Weekday() {
	case time.Monday:
		day = venueFull.Weekdays.Weekdays.Monday
	case time.Tuesday:
		day = venueFull.Weekdays.Weekdays.Tuesday
	case time.Wednesday:
		day = venueFull.Weekdays.Weekdays.Wednesday
	case time.Thursday:
		day = venueFull.Weekdays.Weekdays.Thursday
	case time.Friday:
		day = venueFull.Weekdays.Weekdays.Friday
	case time.Saturday:
		day = venueFull.Weekdays.Weekdays.Saturday
	case time.Sunday:
		day = venueFull.Weekdays.Weekdays.Sunday
	}

	if err := r.checkScheduleMatch(day, finalStartAt, finalEndAt); err != nil {
		return err
	}

	if err := r.checkBookingConflictsTx(r.db, venueID, finalStartAt, finalEndAt, &id); err != nil {
		return err
	}

	return nil
}

// GetVenueAvailability возвращает свободные слоты площадки на дату
func (r *bookingService) GetVenueAvailability(venueID uint, date time.Time) ([]dto.AvailableSlot, error) {
	// Получаем данные площадки с расписанием
	url := fmt.Sprintf("%s/venues/%d", r.venueURL, venueID)
	var venueFull dto.ResponsVenueServFull
	resp, err := r.client.R().SetResult(&venueFull).Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Сервер вернул ошибку: %d", resp.StatusCode())
	}

	// Определяем день недели
	var day dto.DayScheduleDTO
	switch date.Weekday() {
	case time.Monday:
		day = venueFull.Weekdays.Weekdays.Monday
	case time.Tuesday:
		day = venueFull.Weekdays.Weekdays.Tuesday
	case time.Wednesday:
		day = venueFull.Weekdays.Weekdays.Wednesday
	case time.Thursday:
		day = venueFull.Weekdays.Weekdays.Thursday
	case time.Friday:
		day = venueFull.Weekdays.Weekdays.Friday
	case time.Saturday:
		day = venueFull.Weekdays.Weekdays.Saturday
	case time.Sunday:
		day = venueFull.Weekdays.Weekdays.Sunday
	}

	if !day.Enabled {
		// площадка не работает в этот день — нет слотов
		return []dto.AvailableSlot{}, nil
	}

	if day.StartTime == nil || day.EndTime == nil {
		return nil, fmt.Errorf("в расписании площадки отсутствует время работы для выбранного дня")
	}

	tStart, err := time.Parse("15:04", *day.StartTime)
	if err != nil {
		return nil, fmt.Errorf("неверный формат start_time в расписании площадки: %w", err)
	}
	tEnd, err := time.Parse("15:04", *day.EndTime)
	if err != nil {
		return nil, fmt.Errorf("неверный формат end_time в расписании площадки: %w", err)
	}

	venueStart := time.Date(date.Year(), date.Month(), date.Day(), tStart.Hour(), tStart.Minute(), 0, 0, time.UTC)
	venueEnd := time.Date(date.Year(), date.Month(), date.Day(), tEnd.Hour(), tEnd.Minute(), 0, 0, time.UTC)

	// Получаем все брони площадки
	bookings, err := r.repo.GetVenueBookings(venueID)
	if err != nil {
		return nil, err
	}

	// Собираем интервал занятых времён в рабочем дне
	type interval struct {
		start time.Time
		end   time.Time
	}

	var intervals []interval

	for _, b := range bookings {
		if b.Status == models.Cancelled {
			continue
		}
		// Отбираем брони по дате
		if b.StartAt.Year() != date.Year() || b.StartAt.YearDay() != date.YearDay() {
			continue
		}
		// Если бронь вне рабочего времени — пропускаем или обрезаем
		if b.EndAt.Before(venueStart) || b.StartAt.After(venueEnd) {
			continue
		}
		s := b.StartAt
		if s.Before(venueStart) {
			s = venueStart
		}
		e := b.EndAt
		if e.After(venueEnd) {
			e = venueEnd
		}
		intervals = append(intervals, interval{start: s, end: e})
	}

	// Сортируем по start
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].start.Before(intervals[j].start)
	})

	// Сливаем перекрывающиеся интервалы и определяем свободные промежутки
	var slots []dto.AvailableSlot
	prev := venueStart
	for _, it := range intervals {
		if it.start.After(prev) {
			slots = append(slots, dto.AvailableSlot{StartAt: prev, EndAt: it.start})
		}
		if it.end.After(prev) {
			prev = it.end
		}
	}

	if prev.Before(venueEnd) {
		slots = append(slots, dto.AvailableSlot{StartAt: prev, EndAt: venueEnd})
	}

	// Если нет броней — вернуть один слот рабочего времени (если длительность >= 1ч)
	if len(intervals) == 0 {
		if venueEnd.Sub(venueStart) >= time.Hour {
			return []dto.AvailableSlot{{StartAt: venueStart, EndAt: venueEnd}}, nil
		}
		return []dto.AvailableSlot{}, nil
	}

	// Отфильтруем слоты короче 1 часа (минимальная длительность брони)
	var filtered []dto.AvailableSlot
	for _, s := range slots {
		if s.EndAt.Sub(s.StartAt) >= time.Hour {
			filtered = append(filtered, s)
		}
	}

	return filtered, nil
}
