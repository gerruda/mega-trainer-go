package postgres

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"mega-trainer-go/internal/domain"
)

// MeasurementRepo is the PostgreSQL implementation of
// domain.MeasurementRepository.
type MeasurementRepo struct {
	db *sqlx.DB
}

var _ domain.MeasurementRepository = (*MeasurementRepo)(nil)

// NewMeasurementRepo creates a measurement repository.
func NewMeasurementRepo(db *sqlx.DB) *MeasurementRepo {
	return &MeasurementRepo{db: db}
}

const insertMeasurementQuery = `
	INSERT INTO measurements (
		user_id, weight_kg, body_fat_pct, neck_cm, waist_cm, hips_cm, measured_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id, user_id, weight_kg, body_fat_pct, neck_cm, waist_cm, hips_cm,
	          measured_at, created_at
`

// Create stores a new measurement.
func (r *MeasurementRepo) Create(ctx context.Context, measurement *domain.Measurement) (*domain.Measurement, error) {
	var stored domain.Measurement
	err := r.db.GetContext(ctx, &stored, insertMeasurementQuery,
		measurement.UserID,
		measurement.WeightKG,
		nullableFloat(measurement.BodyFatPct),
		nullableFloat(measurement.NeckCM),
		nullableFloat(measurement.WaistCM),
		nullableFloat(measurement.HipsCM),
		measurement.MeasuredAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create measurement for user %d: %w", measurement.UserID, err)
	}

	return &stored, nil
}

const listMeasurementsQuery = `
	SELECT id, user_id, weight_kg, body_fat_pct, neck_cm, waist_cm, hips_cm,
	       measured_at, created_at
	FROM measurements
	WHERE user_id = $1
	ORDER BY measured_at DESC, id DESC
	LIMIT $2
`

// ListByUserID returns the latest measurements of the user, newest first.
func (r *MeasurementRepo) ListByUserID(ctx context.Context, userID int64, limit int) ([]*domain.Measurement, error) {
	var measurements []*domain.Measurement
	if err := r.db.SelectContext(ctx, &measurements, listMeasurementsQuery, userID, limit); err != nil {
		return nil, fmt.Errorf("list measurements for user %d: %w", userID, err)
	}

	return measurements, nil
}

// nullableFloat converts an optional value into a database argument.
func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
