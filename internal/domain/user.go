// Package domain contains the core entities, value objects and repository
// contracts of the application. It has no dependencies on other project
// packages: infrastructure and delivery layers depend on it, never the other
// way around.
package domain

import (
	"database/sql"
	"errors"
	"time"
)

// Sentinel errors used by repositories and services. Callers must compare them
// with errors.Is, because implementations wrap them with additional context.
var (
	ErrNotFound   = errors.New("domain: not found")
	ErrDuplicate  = errors.New("domain: duplicate")
	ErrValidation = errors.New("domain: validation failed")
)

// User is a Telegram user known to the bot.
type User struct {
	ID           int64        `db:"id" json:"id"`
	TelegramID   int64        `db:"telegram_id" json:"telegram_id"`
	Username     string       `db:"username" json:"username"`
	FirstName    string       `db:"first_name" json:"first_name"`
	LastName     string       `db:"last_name" json:"last_name"`
	LanguageCode string       `db:"language_code" json:"language_code"`
	IsActive     bool         `db:"is_active" json:"is_active"`
	CreatedAt    time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at" json:"updated_at"`
	DeletedAt    sql.NullTime `db:"deleted_at" json:"-"`
}

// DisplayName returns the most human-friendly name available for the user.
func (u User) DisplayName() string {
	switch {
	case u.FirstName != "" && u.LastName != "":
		return u.FirstName + " " + u.LastName
	case u.FirstName != "":
		return u.FirstName
	case u.Username != "":
		return "@" + u.Username
	default:
		return "друг"
	}
}

// UserState is a single step of a step-by-step (FSM) dialog. Steps are
// identified by Command, and Data keeps the answers collected so far.
type UserState struct {
	UserID  int64             `json:"user_id"`
	Command string            `json:"command"`
	Step    int               `json:"step"`
	Data    map[string]string `json:"data,omitempty"`
}

// Calculation input/output value objects. Keeping them in one place protects
// the project from duplicated formulas: there is exactly one implementation of
// every calculation in internal/service.

// WaterCalculation is a daily water intake calculation.
type WaterCalculation struct {
	Weight   float64 `json:"weight"`
	Activity string  `json:"activity"`
	Result   float64 `json:"result"`
}

// OneRepMaxCalculation is a one-repetition-maximum calculation.
type OneRepMaxCalculation struct {
	Weight  float64 `json:"weight"`
	Reps    int     `json:"reps"`
	Formula string  `json:"formula"`
	Result  float64 `json:"result"`
}

// BodyFatCalculation is a body fat percentage calculation (US Navy method).
type BodyFatCalculation struct {
	Gender string  `json:"gender"`
	Age    int     `json:"age"`
	Weight float64 `json:"weight"`
	Height float64 `json:"height"`
	Neck   float64 `json:"neck"`
	Waist  float64 `json:"waist"`
	Hip    float64 `json:"hip"`
	Result float64 `json:"result"`
}

// TDEECalculation is a total daily energy expenditure calculation.
type TDEECalculation struct {
	Gender   string  `json:"gender"`
	Age      int     `json:"age"`
	Weight   float64 `json:"weight"`
	Height   float64 `json:"height"`
	Activity string  `json:"activity"`
	Goal     string  `json:"goal"`
	// BMR is the basal metabolic rate before activity and goal adjustments.
	BMR float64 `json:"bmr"`
	// TDEE is the energy expenditure adjusted for the activity level only.
	TDEE float64 `json:"tdee"`
	// Result is the final daily calorie target adjusted for the goal.
	Result float64 `json:"result"`
}

// Vocabulary shared by the service and delivery layers. Using constants instead
// of raw strings keeps callback payloads and formulas in sync.
const (
	GenderMale   = "male"
	GenderFemale = "female"

	WaterActivityLow    = "low"
	WaterActivityMedium = "medium"
	WaterActivityHigh   = "high"

	ActivitySedentary  = "sedentary"
	ActivityLight      = "light"
	ActivityModerate   = "moderate"
	ActivityActive     = "active"
	ActivityVeryActive = "very_active"

	GoalLoss        = "loss"
	GoalMaintenance = "maintenance"
	GoalGain        = "gain"

	FormulaEpley    = "epley"
	FormulaBrzycki  = "brzycki"
	FormulaLombardi = "lombardi"
)
