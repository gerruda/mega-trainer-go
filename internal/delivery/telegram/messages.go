package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/service"
)

// Prompts shown while collecting answers.
const (
	promptWeight  = "Введите ваш вес в кг (например: 70.5):"
	promptAge     = "Введите ваш возраст (10–120 лет):"
	promptHeight  = "Введите ваш рост в см:"
	promptNeck    = "Введите обхват шеи в см:"
	promptWaist   = "Введите обхват талии в см:"
	promptHip     = "Введите обхват бёдер в см:"
	promptReps    = "Введите количество повторений (1–30):"
	promptFormula = "🏋️ *Одноповторный максимум*\n\nВыберите формулу расчёта:"
	promptWater   = "💧 *Норма воды*\n\nВведите ваш вес в кг:"
	promptTDEE    = "🔥 *Суточный расход калорий*\n\nВведите ваш возраст:"
	promptBodyFat = "📊 *Процент жира*\n\nВведите ваш возраст:"
)

func genderKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Мужской", callbackGender+":"+domain.GenderMale),
			tgbotapi.NewInlineKeyboardButtonData("Женский", callbackGender+":"+domain.GenderFemale),
		),
	)
}

func waterActivityKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Низкая", callbackWaterActivity+":"+domain.WaterActivityLow),
			tgbotapi.NewInlineKeyboardButtonData("Средняя", callbackWaterActivity+":"+domain.WaterActivityMedium),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Высокая", callbackWaterActivity+":"+domain.WaterActivityHigh),
		),
	)
}

func formulaKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Эпли", callbackOneRepFormula+":"+domain.FormulaEpley),
			tgbotapi.NewInlineKeyboardButtonData("Бжицки", callbackOneRepFormula+":"+domain.FormulaBrzycki),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ломбарди", callbackOneRepFormula+":"+domain.FormulaLombardi),
		),
	)
}

func activityKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сидячий", callbackActivity+":"+domain.ActivitySedentary),
			tgbotapi.NewInlineKeyboardButtonData("Легкая", callbackActivity+":"+domain.ActivityLight),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Умеренная", callbackActivity+":"+domain.ActivityModerate),
			tgbotapi.NewInlineKeyboardButtonData("Высокая", callbackActivity+":"+domain.ActivityActive),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Очень высокая", callbackActivity+":"+domain.ActivityVeryActive),
		),
	)
}

func goalKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Похудение", callbackGoal+":"+domain.GoalLoss),
			tgbotapi.NewInlineKeyboardButtonData("Поддержание", callbackGoal+":"+domain.GoalMaintenance),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Набор массы", callbackGoal+":"+domain.GoalGain),
		),
	)
}

var genderNames = map[string]string{
	domain.GenderMale:   "Мужской",
	domain.GenderFemale: "Женский",
}

var waterActivityNames = map[string]string{
	domain.WaterActivityLow:    "Низкая",
	domain.WaterActivityMedium: "Средняя",
	domain.WaterActivityHigh:   "Высокая",
}

var activityNames = map[string]string{
	domain.ActivitySedentary:  "Сидячий образ жизни",
	domain.ActivityLight:      "Лёгкая активность (1–3 тренировки в неделю)",
	domain.ActivityModerate:   "Умеренная активность (3–5 тренировок в неделю)",
	domain.ActivityActive:     "Высокая активность (6–7 тренировок в неделю)",
	domain.ActivityVeryActive: "Очень высокая активность",
}

var goalNames = map[string]string{
	domain.GoalLoss:        "Похудение",
	domain.GoalMaintenance: "Поддержание веса",
	domain.GoalGain:        "Набор массы",
}

var formulaNames = map[string]string{
	domain.FormulaEpley:    "Эпли",
	domain.FormulaBrzycki:  "Бжицки",
	domain.FormulaLombardi: "Ломбарди",
}

func formatWaterResult(result domain.WaterCalculation) string {
	return fmt.Sprintf(
		"💧 *Норма воды*\n\n"+
			"• Вес: %.1f кг\n"+
			"• Активность: %s\n\n"+
			"Рекомендуемая норма: *%.2f л в день*\n\n"+
			"Для нового расчёта используйте команды из меню.",
		result.Weight, waterActivityNames[result.Activity], result.Result,
	)
}

func formatOneRepMaxResult(result domain.OneRepMaxCalculation) string {
	alternatives := fmt.Sprintf(
		"• Эпли: %.1f кг\n• Бжицки: %.1f кг\n• Ломбарди: %.1f кг",
		oneRepMaxValue(result.Weight, result.Reps, domain.FormulaEpley),
		oneRepMaxValue(result.Weight, result.Reps, domain.FormulaBrzycki),
		oneRepMaxValue(result.Weight, result.Reps, domain.FormulaLombardi),
	)

	return fmt.Sprintf(
		"🏋️ *Одноповторный максимум*\n\n"+
			"• Вес: %.1f кг\n"+
			"• Повторений: %d\n"+
			"• Формула: %s\n\n"+
			"Результат: *%.1f кг*\n\n"+
			"*Сравнение формул:*\n%s\n\n"+
			"Для нового расчёта используйте команды из меню.",
		result.Weight, result.Reps, formulaNames[result.Formula], result.Result, alternatives,
	)
}

// oneRepMaxValue recalculates the maximum with another formula for the
// comparison block. Errors are impossible here: the inputs were validated
// before the result screen is rendered.
func oneRepMaxValue(weight float64, reps int, formula string) float64 {
	result, err := service.CalculateOneRepMax(domain.OneRepMaxCalculation{
		Weight:  weight,
		Reps:    reps,
		Formula: formula,
	})
	if err != nil {
		return 0
	}
	return result.Result
}

func formatBodyFatResult(result domain.BodyFatCalculation) string {
	return fmt.Sprintf(
		"📊 *Процент жира*\n\n"+
			"• Пол: %s\n"+
			"• Возраст: %d лет\n"+
			"• Вес: %.1f кг\n"+
			"• Рост: %.1f см\n\n"+
			"Процент жира: *%.1f%%*\n"+
			"Категория: %s\n\n"+
			"Для нового расчёта используйте команды из меню.",
		genderNames[result.Gender],
		result.Age,
		result.Weight,
		result.Height,
		result.Result,
		bodyFatCategory(result),
	)
}

func bodyFatCategory(result domain.BodyFatCalculation) string {
	fat := result.Result

	if result.Gender == domain.GenderMale {
		switch {
		case fat < 6:
			return "❌ Слишком низкий (опасно для здоровья)"
		case fat < 14:
			return "🏆 Атлетическое телосложение"
		case fat < 18:
			return "👍 Спортивное телосложение"
		case fat < 25:
			return "✅ Норма"
		case fat < 30:
			return "⚠️ Избыточный вес"
		default:
			return "🚨 Ожирение"
		}
	}

	switch {
	case fat < 14:
		return "❌ Слишком низкий (опасно для здоровья)"
	case fat < 21:
		return "🏆 Атлетическое телосложение"
	case fat < 25:
		return "👍 Спортивное телосложение"
	case fat < 32:
		return "✅ Норма"
	case fat < 38:
		return "⚠️ Избыточный вес"
	default:
		return "🚨 Ожирение"
	}
}

func formatTDEEResult(result domain.TDEECalculation) string {
	return fmt.Sprintf(
		"🔥 *Суточный расход калорий*\n\n"+
			"• Пол: %s\n"+
			"• Возраст: %d лет\n"+
			"• Вес: %.1f кг\n"+
			"• Рост: %.1f см\n"+
			"• Активность: %s\n"+
			"• Цель: %s\n\n"+
			"Базовый метаболизм (BMR): %.0f ккал\n"+
			"Расход с учётом активности (TDEE): %.0f ккал\n\n"+
			"Ваша норма: *%.0f ккал в день*\n\n"+
			"Для нового расчёта используйте команды из меню.",
		genderNames[result.Gender],
		result.Age,
		result.Weight,
		result.Height,
		activityNames[result.Activity],
		goalNames[result.Goal],
		result.BMR,
		result.TDEE,
		result.Result,
	)
}
