package service

import (
	"errors"
	"math"
	"testing"

	"mega-trainer-go/internal/domain"
)

func TestCalculateWater(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		weight   float64
		activity string
		want     float64
	}{
		{name: "low activity", weight: 70, activity: domain.WaterActivityLow, want: 2.1},
		{name: "medium activity", weight: 70, activity: domain.WaterActivityMedium, want: 2.52},
		{name: "high activity", weight: 70, activity: domain.WaterActivityHigh, want: 3.15},
		{name: "fractional weight", weight: 55.5, activity: domain.WaterActivityMedium, want: 1.998},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CalculateWater(domain.WaterCalculation{Weight: tt.weight, Activity: tt.activity})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got.Result-tt.want) > 1e-9 {
				t.Fatalf("result = %v, want %v", got.Result, tt.want)
			}
		})
	}
}

func TestCalculateWaterValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain.WaterCalculation
	}{
		{name: "zero weight", in: domain.WaterCalculation{Weight: 0, Activity: domain.WaterActivityLow}},
		{name: "unknown activity", in: domain.WaterCalculation{Weight: 70, Activity: "extreme"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := CalculateWater(tt.in); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestCalculateOneRepMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		formula string
		want    float64
	}{
		{name: "epley", formula: domain.FormulaEpley, want: 116.66666666666667},
		{name: "brzycki", formula: domain.FormulaBrzycki, want: 112.5},
		{name: "lombardi", formula: domain.FormulaLombardi, want: 117.4618943088019},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CalculateOneRepMax(domain.OneRepMaxCalculation{
				Weight:  100,
				Reps:    5,
				Formula: tt.formula,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got.Result-tt.want) > 1e-9 {
				t.Fatalf("result = %v, want %v", got.Result, tt.want)
			}
		})
	}
}

func TestCalculateOneRepMaxValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain.OneRepMaxCalculation
	}{
		{name: "zero reps", in: domain.OneRepMaxCalculation{Weight: 100, Reps: 0, Formula: domain.FormulaEpley}},
		{name: "too many reps", in: domain.OneRepMaxCalculation{Weight: 100, Reps: 31, Formula: domain.FormulaEpley}},
		{name: "zero weight", in: domain.OneRepMaxCalculation{Weight: 0, Reps: 5, Formula: domain.FormulaEpley}},
		{name: "unknown formula", in: domain.OneRepMaxCalculation{Weight: 100, Reps: 5, Formula: "unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := CalculateOneRepMax(tt.in); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestCalculateBodyFatMale(t *testing.T) {
	t.Parallel()

	got, err := CalculateBodyFat(domain.BodyFatCalculation{
		Gender: domain.GenderMale,
		Height: 180,
		Neck:   40,
		Waist:  90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reference value for the US Navy formula: ~18.4 %.
	if math.Abs(got.Result-18.4) > 0.1 {
		t.Fatalf("result = %.2f, want ~18.4", got.Result)
	}
}

func TestCalculateBodyFatFemale(t *testing.T) {
	t.Parallel()

	got, err := CalculateBodyFat(domain.BodyFatCalculation{
		Gender: domain.GenderFemale,
		Height: 170,
		Neck:   32,
		Waist:  75,
		Hip:    95,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reference value for the US Navy formula: ~26.1 %.
	if math.Abs(got.Result-26.1) > 0.2 {
		t.Fatalf("result = %.2f, want ~26.1", got.Result)
	}
}

func TestCalculateBodyFatValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain.BodyFatCalculation
	}{
		{
			name: "male waist not greater than neck",
			in:   domain.BodyFatCalculation{Gender: domain.GenderMale, Height: 180, Neck: 40, Waist: 40},
		},
		{
			name: "female without hip",
			in:   domain.BodyFatCalculation{Gender: domain.GenderFemale, Height: 170, Neck: 32, Waist: 75},
		},
		{
			name: "unknown gender",
			in:   domain.BodyFatCalculation{Gender: "other", Height: 170, Neck: 32, Waist: 75},
		},
		{
			name: "zero height",
			in:   domain.BodyFatCalculation{Gender: domain.GenderMale, Height: 0, Neck: 40, Waist: 90},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := CalculateBodyFat(tt.in); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestCalculateTDEE(t *testing.T) {
	t.Parallel()

	base := domain.TDEECalculation{
		Gender:   domain.GenderMale,
		Age:      30,
		Weight:   80,
		Height:   180,
		Activity: domain.ActivityModerate,
	}

	t.Run("bmr and tdee", func(t *testing.T) {
		t.Parallel()

		got, err := CalculateTDEE(withGoal(base, domain.GoalMaintenance))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.BMR != 1780 {
			t.Fatalf("bmr = %v, want 1780", got.BMR)
		}
		if math.Abs(got.Result-2759) > 1e-9 {
			t.Fatalf("result = %v, want 2759", got.Result)
		}
	})

	t.Run("loss and gain adjust the target", func(t *testing.T) {
		t.Parallel()

		loss, err := CalculateTDEE(withGoal(base, domain.GoalLoss))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(loss.Result-2759*0.85) > 1e-9 {
			t.Fatalf("loss result = %v, want %v", loss.Result, 2759*0.85)
		}

		gain, err := CalculateTDEE(withGoal(base, domain.GoalGain))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(gain.Result-2759*1.15) > 1e-9 {
			t.Fatalf("gain result = %v, want %v", gain.Result, 2759*1.15)
		}
	})
}

func TestCalculateTDEEValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain.TDEECalculation
	}{
		{
			name: "unknown gender",
			in: domain.TDEECalculation{
				Gender: "other", Age: 30, Weight: 80, Height: 180,
				Activity: domain.ActivityModerate, Goal: domain.GoalMaintenance,
			},
		},
		{
			name: "unknown activity",
			in: domain.TDEECalculation{
				Gender: domain.GenderMale, Age: 30, Weight: 80, Height: 180,
				Activity: "unknown", Goal: domain.GoalMaintenance,
			},
		},
		{
			name: "unknown goal",
			in: domain.TDEECalculation{
				Gender: domain.GenderMale, Age: 30, Weight: 80, Height: 180,
				Activity: domain.ActivityModerate, Goal: "unknown",
			},
		},
		{
			name: "unrealistic age",
			in: domain.TDEECalculation{
				Gender: domain.GenderMale, Age: 5, Weight: 80, Height: 180,
				Activity: domain.ActivityModerate, Goal: domain.GoalMaintenance,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := CalculateTDEE(tt.in); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func withGoal(in domain.TDEECalculation, goal string) domain.TDEECalculation {
	in.Goal = goal
	return in
}
