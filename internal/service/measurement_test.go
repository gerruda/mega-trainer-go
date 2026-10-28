package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"mega-trainer-go/internal/domain"
)

// memMeasurementRepo is an in-memory domain.MeasurementRepository for tests.
type memMeasurementRepo struct {
	items []domain.Measurement
	next  int64
}

func (r *memMeasurementRepo) Create(_ context.Context, measurement *domain.Measurement) (*domain.Measurement, error) {
	r.next++

	stored := *measurement
	stored.ID = r.next
	stored.CreatedAt = stored.MeasuredAt
	r.items = append(r.items, stored)

	return &stored, nil
}

func (r *memMeasurementRepo) ListByUserID(_ context.Context, userID int64, limit int) ([]*domain.Measurement, error) {
	var matched []*domain.Measurement
	for i := len(r.items) - 1; i >= 0; i-- {
		if r.items[i].UserID != userID {
			continue
		}
		matched = append(matched, &r.items[i])
		if limit > 0 && len(matched) == limit {
			break
		}
	}
	return matched, nil
}

func TestMeasurementServiceRecordAndRead(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	users := newMemUserRepo()
	repo := &memMeasurementRepo{}
	svc := NewMeasurementService(repo, users)

	if _, err := users.Upsert(ctx, &domain.User{TelegramID: 555}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	stored, err := svc.Record(ctx, 555, domain.Measurement{WeightKG: 70.5})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if stored.UserID != 1 {
		t.Fatalf("stored user id = %d, want the internal user id 1", stored.UserID)
	}
	if stored.MeasuredAt.IsZero() {
		t.Fatal("measured_at was not set")
	}

	if _, err := svc.Record(ctx, 555, domain.Measurement{WeightKG: 69.8}); err != nil {
		t.Fatalf("record second: %v", err)
	}

	recent, err := svc.Recent(ctx, 555, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("recent len = %d, want 2", len(recent))
	}
	// Newest first.
	if recent[0].WeightKG != 69.8 {
		t.Fatalf("recent[0] = %v, want 69.8", recent[0].WeightKG)
	}

	series, err := svc.WeightSeries(ctx, 555, 10)
	if err != nil {
		t.Fatalf("weight series: %v", err)
	}
	// Chronological order for plotting.
	if len(series) != 2 || series[0] != 70.5 || series[1] != 69.8 {
		t.Fatalf("series = %v, want [70.5 69.8]", series)
	}
}

func TestMeasurementServiceUnknownUserHasNoData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := NewMeasurementService(&memMeasurementRepo{}, newMemUserRepo())

	recent, err := svc.Recent(ctx, 404, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if recent != nil {
		t.Fatalf("recent = %v, want nil", recent)
	}
}

func TestMeasurementServiceValidatesWeight(t *testing.T) {
	t.Parallel()

	svc := NewMeasurementService(&memMeasurementRepo{}, newMemUserRepo())

	for _, weight := range []float64{0, -1, 19.9, 400.1} {
		_, err := svc.Record(context.Background(), 1, domain.Measurement{WeightKG: weight})
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("weight %v: error = %v, want ErrValidation", weight, err)
		}
	}
}

func TestMeasurementServiceRequiresKnownUser(t *testing.T) {
	t.Parallel()

	svc := NewMeasurementService(&memMeasurementRepo{}, newMemUserRepo())

	_, err := svc.Record(context.Background(), 404, domain.Measurement{WeightKG: 70})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestMeasurementLimitIsClamped(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	users := newMemUserRepo()
	repo := &memMeasurementRepo{}
	svc := NewMeasurementService(repo, users)

	if _, err := users.Upsert(ctx, &domain.User{TelegramID: 777}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.Record(ctx, 777, domain.Measurement{
			WeightKG:   70 + float64(i),
			MeasuredAt: time.Now().UTC().Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	recent, err := svc.Recent(ctx, 777, 0) // default limit
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("recent len = %d, want 3", len(recent))
	}
}
