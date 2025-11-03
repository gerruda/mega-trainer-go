package calculator

import (
	"math"
)

// CalculateWater рассчитывает норму воды
func CalculateWater(weight float64, activityLevel string) float64 {
	baseWater := weight * 0.03 // 30 мл на кг веса

	switch activityLevel {
	case "low":
		return baseWater
	case "medium":
		return baseWater * 1.2
	case "high":
		return baseWater * 1.5
	default:
		return baseWater
	}
}

// CalculateOneRepMax рассчитывает одноповторный максимум
func CalculateOneRepMax(weight float64, reps int) float64 {
	// Формула Эйпли
	return weight * (1 + float64(reps)/30)
}

// TDEE (упрощённо: BMR × активность)
func CalculateTDEE(gender string, weight, height, age float64, activity float64) float64 {
	var bmr float64
	if gender == "male" {
		bmr = 88.362 + (13.397 * weight) + (4.799 * height) - (5.677 * age)
	} else {
		bmr = 447.593 + (9.247 * weight) + (3.098 * height) - (4.330 * age)
	}
	return bmr * activity // activity: 1.2–1.9
}

// CalculateBMI рассчитывает индекс массы тела
func CalculateBMI(height, weight float64) (bmi float64, category string) {
	heightInMeters := height / 100
	bmi = weight / (heightInMeters * heightInMeters)

	switch {
	case bmi < 18.5:
		category = "Недостаточный вес"
	case bmi < 25:
		category = "Нормальный вес"
	case bmi < 30:
		category = "Избыточный вес"
	default:
		category = "Ожирение"
	}

	return
}

// CalculateBodyFat рассчитывает процент жира в организме
func CalculateBodyFat(gender string, age int, height, weight, neck, waist, hips float64) float64 {
	// Формула ВМС США
	if gender == "male" {
		return 495/(1.0324-0.19077*math.Log10(waist-neck)+0.15456*math.Log10(height)) - 450
	} else {
		return 495/(1.29579-0.35004*math.Log10(waist+hips-neck)+0.22100*math.Log10(height)) - 450
	}
}

// CalculateCalories рассчитывает норму калорий и БЖУ
func CalculateCalories(gender string, age int, height, weight float64, activityLevel, goal string) (bmr, tdee float64, calories, protein, fat, carbs int) {
	// Формула Миффлина-Сан Жеора
	if gender == "male" {
		bmr = 10*weight + 6.25*height - 5*float64(age) + 5
	} else {
		bmr = 10*weight + 6.25*height - 5*float64(age) - 161
	}

	// Коэффициент активности
	activityMultiplier := getActivityMultiplier(activityLevel)
	tdee = bmr * activityMultiplier

	// Корректировка по цели
	switch goal {
	case "lose":
		calories = int(tdee * 0.85)
	case "gain":
		calories = int(tdee * 1.15)
	default:
		calories = int(tdee)
	}

	// Расчет БЖУ
	protein = int(weight * 2.2)             // 2.2г белка на кг веса
	fat = int(float64(calories) * 0.25 / 9) // 25% от калорий, 9 ккал/г
	carbs = int((float64(calories) - float64(protein*4) - float64(fat*9)) / 4)

	return
}

func getActivityMultiplier(level string) float64 {
	switch level {
	case "sedentary":
		return 1.2
	case "light":
		return 1.375
	case "moderate":
		return 1.55
	case "active":
		return 1.725
	case "very_active":
		return 1.9
	default:
		return 1.2
	}
}
