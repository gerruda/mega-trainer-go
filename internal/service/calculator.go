// Package service contains the application's business logic: fitness formula
// calculations, the dialog state machine helpers and user management. It
// depends on domain contracts only.
package service

import (
	"errors"
	"fmt"
	"math"

	"mega-trainer-go/internal/domain"
)

// Water intake constants: 30 ml per kilogram of body weight, adjusted for the
// reported activity level.
const waterBasePerKG = 0.03

// CalculateWater returns the recommended daily water intake in litres.
func CalculateWater(in domain.WaterCalculation) (domain.WaterCalculation, error) {
	if in.Weight <= 0 {
		return in, fmt.Errorf("%w: weight must be positive", domain.ErrValidation)
	}

	var multiplier float64
	switch in.Activity {
	case domain.WaterActivityLow:
		multiplier = 1.0
	case domain.WaterActivityMedium:
		multiplier = 1.2
	case domain.WaterActivityHigh:
		multiplier = 1.5
	default:
		return in, fmt.Errorf("%w: unknown water activity %q", domain.ErrValidation, in.Activity)
	}

	in.Result = in.Weight * waterBasePerKG * multiplier
	return in, nil
}

// CalculateOneRepMax returns the estimated one-repetition maximum.
func CalculateOneRepMax(in domain.OneRepMaxCalculation) (domain.OneRepMaxCalculation, error) {
	if in.Weight <= 0 {
		return in, fmt.Errorf("%w: weight must be positive", domain.ErrValidation)
	}
	if in.Reps < 1 || in.Reps > 30 {
		return in, fmt.Errorf("%w: reps must be between 1 and 30", domain.ErrValidation)
	}

	reps := float64(in.Reps)
	switch in.Formula {
	case domain.FormulaEpley:
		in.Result = in.Weight * (1 + reps/30)
	case domain.FormulaBrzycki:
		// The original formula is undefined for 37 repetitions and above; the
		// rep range is validated before this point.
		in.Result = in.Weight * (36 / (37 - reps))
	case domain.FormulaLombardi:
		in.Result = in.Weight * math.Pow(reps, 0.1)
	default:
		return in, fmt.Errorf("%w: unknown one-rep-max formula %q", domain.ErrValidation, in.Formula)
	}

	return in, nil
}

// CalculateBodyFat returns the body fat percentage using the US Navy method.
// Heights and circumferences are in centimetres.
func CalculateBodyFat(in domain.BodyFatCalculation) (domain.BodyFatCalculation, error) {
	if in.Height <= 0 || in.Neck <= 0 || in.Waist <= 0 {
		return in, fmt.Errorf("%w: height, neck and waist must be positive", domain.ErrValidation)
	}

	switch in.Gender {
	case domain.GenderMale:
		// log10 of a non-positive number is undefined, so the input geometry is
		// validated explicitly instead of producing NaN/Inf results.
		if in.Waist <= in.Neck {
			return in, fmt.Errorf("%w: waist must be greater than neck", domain.ErrValidation)
		}
		in.Result = 495/(1.0324-
			0.19077*math.Log10(in.Waist-in.Neck)+
			0.15456*math.Log10(in.Height)) - 450
	case domain.GenderFemale:
		if in.Hip <= 0 {
			return in, fmt.Errorf("%w: hip must be positive for female calculation", domain.ErrValidation)
		}
		if in.Waist+in.Hip <= in.Neck {
			return in, fmt.Errorf("%w: waist + hip must be greater than neck", domain.ErrValidation)
		}
		in.Result = 495/(1.29579-
			0.35004*math.Log10(in.Waist+in.Hip-in.Neck)+
			0.22100*math.Log10(in.Height)) - 450
	default:
		return in, fmt.Errorf("%w: unknown gender %q", domain.ErrValidation, in.Gender)
	}

	if math.IsNaN(in.Result) || math.IsInf(in.Result, 0) {
		return in, errors.New("calculation produced a non-finite result")
	}

	return in, nil
}

// CalculateTDEE returns BMR and the daily calorie target using the
// Mifflin-St Jeor equation adjusted for activity level and goal.
func CalculateTDEE(in domain.TDEECalculation) (domain.TDEECalculation, error) {
	if in.Weight <= 0 || in.Height <= 0 {
		return in, fmt.Errorf("%w: weight and height must be positive", domain.ErrValidation)
	}
	if in.Age < 10 || in.Age > 120 {
		return in, fmt.Errorf("%w: age must be between 10 and 120", domain.ErrValidation)
	}

	switch in.Gender {
	case domain.GenderMale:
		in.BMR = 10*in.Weight + 6.25*in.Height - 5*float64(in.Age) + 5
	case domain.GenderFemale:
		in.BMR = 10*in.Weight + 6.25*in.Height - 5*float64(in.Age) - 161
	default:
		return in, fmt.Errorf("%w: unknown gender %q", domain.ErrValidation, in.Gender)
	}

	multipliers := map[string]float64{
		domain.ActivitySedentary:  1.2,
		domain.ActivityLight:      1.375,
		domain.ActivityModerate:   1.55,
		domain.ActivityActive:     1.725,
		domain.ActivityVeryActive: 1.9,
	}
	multiplier, ok := multipliers[in.Activity]
	if !ok {
		return in, fmt.Errorf("%w: unknown activity level %q", domain.ErrValidation, in.Activity)
	}
	in.TDEE = in.BMR * multiplier

	switch in.Goal {
	case domain.GoalLoss:
		in.Result = in.TDEE * 0.85
	case domain.GoalMaintenance:
		in.Result = in.TDEE
	case domain.GoalGain:
		in.Result = in.TDEE * 1.15
	default:
		return in, fmt.Errorf("%w: unknown goal %q", domain.ErrValidation, in.Goal)
	}

	return in, nil
}
