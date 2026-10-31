package domain

import (
	"context"
	"time"
)

// Measurement is a body measurement recorded for a user. Optional fields are
// pointers so "not measured" and "measured as zero" stay distinguishable.
type Measurement struct {
	ID         int64     `db:"id" json:"id"`
	UserID     int64     `db:"user_id" json:"user_id"`
	WeightKG   float64   `db:"weight_kg" json:"weight_kg"`
	BodyFatPct *float64  `db:"body_fat_pct" json:"body_fat_pct,omitempty"`
	NeckCM     *float64  `db:"neck_cm" json:"neck_cm,omitempty"`
	WaistCM    *float64  `db:"waist_cm" json:"waist_cm,omitempty"`
	HipsCM     *float64  `db:"hips_cm" json:"hips_cm,omitempty"`
	MeasuredAt time.Time `db:"measured_at" json:"measured_at"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// MeasurementRepository persists body measurements.
type MeasurementRepository interface {
	Create(ctx context.Context, measurement *Measurement) (*Measurement, error)
	// ListByUserID returns the latest measurements of the user, newest first.
	ListByUserID(ctx context.Context, userID int64, limit int) ([]*Measurement, error)
}
