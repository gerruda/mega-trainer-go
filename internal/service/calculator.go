package service

import (
	"math"
	"mega-trainer-go/internal/domain"
)

type CalculatorService struct{}

func NewCalculatorService() *CalculatorService {
	return &CalculatorService{}
}

func (s *CalculatorService) CalculateWater(req *domain.WaterCalculation) *domain.WaterCalculation {
	baseWater := req.Weight * 0.03

	activityMultiplier := 1.0
	switch req.Activity {
	case "low":
		activityMultiplier = 1.0
	case "medium":
		activityMultiplier = 1.2
	case "high":
		activityMultiplier = 1.5
	}

	req.Result = baseWater * activityMultiplier
	return req
}

func (s *CalculatorService) CalculateOneRepMax(req *domain.OneRepMaxCalculation) *domain.OneRepMaxCalculation {
	switch req.Formula {
	case "epley":
		req.Result = req.Weight * (1 + float64(req.Reps)/30)
	case "brzycki":
		req.Result = req.Weight * (36.0 / (37.0 - float64(req.Reps)))
	case "lombardi":
		req.Result = req.Weight * math.Pow(float64(req.Reps), 0.1)
	default: // epley по умолчанию
		req.Result = req.Weight * (1 + float64(req.Reps)/30)
	}
	return req
}

func (s *CalculatorService) CalculateBodyFat(req *domain.BodyFatCalculation) *domain.BodyFatCalculation {
	if req.Gender == "male" {
		// Формула для мужчин (Navy Method)
		req.Result = 495/(1.0324-0.19077*math.Log10(req.Waist-req.Neck)+0.15456*math.Log10(req.Height)) - 450
	} else {
		// Формула для женщин (Navy Method)
		req.Result = 495/(1.29579-0.35004*math.Log10(req.Waist+req.Hip-req.Neck)+0.22100*math.Log10(req.Height)) - 450
	}
	return req
}

func (s *CalculatorService) CalculateTDEE(req *domain.TDEECalculation) *domain.TDEECalculation {
	// Расчет BMR (Mifflin-St Jeor Formula)
	var bmr float64
	if req.Gender == "male" {
		bmr = 10*req.Weight + 6.25*req.Height - 5*float64(req.Age) + 5
	} else {
		bmr = 10*req.Weight + 6.25*req.Height - 5*float64(req.Age) - 161
	}

	// Умножение на коэффициент активности
	activityMultipliers := map[string]float64{
		"sedentary":   1.2,
		"light":       1.375,
		"moderate":    1.55,
		"active":      1.725,
		"very_active": 1.9,
	}

	tdee := bmr * activityMultipliers[req.Activity]

	// Корректировка по цели
	switch req.Goal {
	case "loss":
		tdee *= 0.85
	case "gain":
		tdee *= 1.15
	}

	req.Result = tdee
	return req
}
