package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mega-trainer-go/internal/domain"
)

// Bounds for recorded measurements.
const (
	minMeasurementWeightKG = 20
	maxMeasurementWeightKG = 400
	maxMeasurementLimit    = 1000
)

// MeasurementService records body measurements and reads them back. It resolves
// the Telegram identity to the internal user id, so the delivery layer never
// works with database ids.
type MeasurementService struct {
	measurements domain.MeasurementRepository
	users        domain.UserRepository
}

// NewMeasurementService creates a MeasurementService.
func NewMeasurementService(measurements domain.MeasurementRepository, users domain.UserRepository) *MeasurementService {
	return &MeasurementService{measurements: measurements, users: users}
}

// Record stores a measurement for the user identified by the Telegram id.
func (s *MeasurementService) Record(ctx context.Context, telegramID int64, measurement domain.Measurement) (*domain.Measurement, error) {
	if measurement.WeightKG < minMeasurementWeightKG || measurement.WeightKG > maxMeasurementWeightKG {
		return nil, fmt.Errorf("%w: weight must be between %d and %d kg",
			domain.ErrValidation, minMeasurementWeightKG, maxMeasurementWeightKG)
	}

	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("resolve user %d: %w", telegramID, err)
	}

	measurement.UserID = user.ID
	measurement.ID = 0
	if measurement.MeasuredAt.IsZero() {
		measurement.MeasuredAt = time.Now().UTC()
	}

	stored, err := s.measurements.Create(ctx, &measurement)
	if err != nil {
		return nil, fmt.Errorf("record measurement: %w", err)
	}

	return stored, nil
}

// Recent returns the latest measurements of the user, newest first. An unknown
// user simply has no measurements.
func (s *MeasurementService) Recent(ctx context.Context, telegramID int64, limit int) ([]domain.Measurement, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > maxMeasurementLimit {
		limit = maxMeasurementLimit
	}

	user, err := s.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve user %d: %w", telegramID, err)
	}

	stored, err := s.measurements.ListByUserID(ctx, user.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("list measurements: %w", err)
	}

	measurements := make([]domain.Measurement, 0, len(stored))
	for _, measurement := range stored {
		measurements = append(measurements, *measurement)
	}

	return measurements, nil
}

// WeightSeries returns the weights in chronological order, ready for plotting.
func (s *MeasurementService) WeightSeries(ctx context.Context, telegramID int64, limit int) ([]float64, error) {
	measurements, err := s.Recent(ctx, telegramID, limit)
	if err != nil {
		return nil, err
	}

	weights := make([]float64, 0, len(measurements))
	for i := len(measurements) - 1; i >= 0; i-- {
		weights = append(weights, measurements[i].WeightKG)
	}

	return weights, nil
}
