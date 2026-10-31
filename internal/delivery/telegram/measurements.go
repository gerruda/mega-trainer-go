package telegram

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/plot"
)

// How many rows each measurement screen works with.
const (
	chartPoints = 30
	historyRows = 10
	exportRows  = 1000
	dateLayout  = "02.01.2006 15:04"
)

func (h *Handler) startMeasure(ctx context.Context, chatID, userID int64) error {
	if h.measurements == nil {
		return h.sendError(chatID, "Дневник измерений недоступен.")
	}

	if err := h.state.Start(ctx, userID, commandMeasure); err != nil {
		return err
	}
	return h.send(chatID, "⚖️ Введите ваш вес в кг (например: 70.5):")
}

func (h *Handler) handleMeasureState(ctx context.Context, chatID, userID int64, text string, _ *domain.UserState) error {
	weight, ok := parseDecimal(text, minWeightKG, maxWeightKG)
	if !ok {
		return h.sendError(chatID, fmt.Sprintf("Введите вес числом от %d до %d кг.", minWeightKG, maxWeightKG))
	}

	stored, err := h.measurements.Record(ctx, userID, domain.Measurement{WeightKG: weight})
	if err != nil {
		h.log.Error().Err(err).Int64("telegram_id", userID).Msg(logger.Source(logComponent, "handleMeasureState") + "не удалось сохранить измерение")
		return h.sendError(chatID, "Не удалось сохранить измерение. Попробуйте позже.")
	}

	if err := h.state.Finish(ctx, userID); err != nil {
		return err
	}

	return h.send(chatID, fmt.Sprintf(
		"✅ Записано: *%.1f кг*.\n\n📈 /chart — динамика веса\n📋 /history — последние измерения",
		stored.WeightKG,
	))
}

func (h *Handler) showWeightChart(ctx context.Context, chatID, userID int64) error {
	if h.measurements == nil {
		return h.sendError(chatID, "Дневник измерений недоступен.")
	}

	weights, err := h.measurements.WeightSeries(ctx, userID, chartPoints)
	if err != nil {
		h.log.Error().Err(err).Int64("telegram_id", userID).Msg(logger.Source(logComponent, "showWeightChart") + "не удалось получить ряд весов")
		return h.sendError(chatID, "Не удалось получить измерения. Попробуйте позже.")
	}

	if len(weights) < 2 {
		return h.send(chatID, "Для графика нужно минимум два измерения.\nЗапишите вес командой /measure.")
	}

	png, err := plot.RenderLineChart(weights)
	if err != nil {
		h.log.Error().Err(err).Msg(logger.Source(logComponent, "showWeightChart") + "не удалось построить график веса")
		return h.sendError(chatID, "Не удалось построить график. Попробуйте позже.")
	}

	minWeight, maxWeight := weights[0], weights[0]
	for _, weight := range weights[1:] {
		if weight < minWeight {
			minWeight = weight
		}
		if weight > maxWeight {
			maxWeight = weight
		}
	}

	caption := fmt.Sprintf(
		"📈 *Динамика веса*\n\n"+
			"• Измерений: %d\n"+
			"• Сейчас: %.1f кг\n"+
			"• Минимум: %.1f кг\n"+
			"• Максимум: %.1f кг",
		len(weights), weights[len(weights)-1], minWeight, maxWeight,
	)

	return h.sendPhoto(chatID, png, caption)
}

func (h *Handler) showHistory(ctx context.Context, chatID, userID int64) error {
	if h.measurements == nil {
		return h.sendError(chatID, "Дневник измерений недоступен.")
	}

	measurements, err := h.measurements.Recent(ctx, userID, historyRows)
	if err != nil {
		h.log.Error().Err(err).Int64("telegram_id", userID).Msg(logger.Source(logComponent, "showHistory") + "не удалось загрузить измерения")
		return h.sendError(chatID, "Не удалось получить измерения. Попробуйте позже.")
	}

	if len(measurements) == 0 {
		return h.send(chatID, "Пока нет измерений. Запишите вес командой /measure.")
	}

	var text strings.Builder
	text.WriteString("📋 *Последние измерения:*\n\n")
	for _, measurement := range measurements {
		text.WriteString("• ")
		text.WriteString(measurement.MeasuredAt.Local().Format(dateLayout))
		fmt.Fprintf(&text, " — *%.1f кг*", measurement.WeightKG)
		if measurement.BodyFatPct != nil {
			fmt.Fprintf(&text, ", %.1f%% жира", *measurement.BodyFatPct)
		}
		text.WriteString("\n")
	}
	text.WriteString("\n📈 /chart — график динамики")

	return h.send(chatID, text.String())
}

func (h *Handler) exportMeasurements(ctx context.Context, chatID, userID int64) error {
	if h.measurements == nil {
		return h.sendError(chatID, "Дневник измерений недоступен.")
	}

	measurements, err := h.measurements.Recent(ctx, userID, exportRows)
	if err != nil {
		h.log.Error().Err(err).Int64("telegram_id", userID).Msg(logger.Source(logComponent, "exportMeasurements") + "не удалось загрузить измерения для экспорта")
		return h.sendError(chatID, "Не удалось получить измерения. Попробуйте позже.")
	}

	if len(measurements) == 0 {
		return h.send(chatID, "Пока нет данных для выгрузки. Запишите вес командой /measure.")
	}

	data, err := measurementsCSV(measurements)
	if err != nil {
		h.log.Error().Err(err).Msg(logger.Source(logComponent, "exportMeasurements") + "не удалось сформировать CSV-файл")
		return h.sendError(chatID, "Не удалось подготовить файл. Попробуйте позже.")
	}

	return h.sendDocument(chatID, "measurements.csv", data,
		fmt.Sprintf("📄 Выгрузка измерений: %d записей", len(measurements)))
}

// recordBodyFatMeasurement stores the data collected by the /bodyfat dialog.
// A persistence failure is logged but never breaks the calculation result.
func (h *Handler) recordBodyFatMeasurement(ctx context.Context, userID int64, calculation domain.BodyFatCalculation, result float64) bool {
	if h.measurements == nil {
		return false
	}

	measurement := domain.Measurement{
		WeightKG:   calculation.Weight,
		BodyFatPct: &result,
		NeckCM:     &calculation.Neck,
		WaistCM:    &calculation.Waist,
	}
	if calculation.Gender == domain.GenderFemale {
		hip := calculation.Hip
		measurement.HipsCM = &hip
	}

	if _, err := h.measurements.Record(ctx, userID, measurement); err != nil {
		h.log.Warn().Err(err).Int64("telegram_id", userID).Msg(logger.Source(logComponent, "recordBodyFatMeasurement") + "не удалось сохранить измерение после расчёта процента жира")
		return false
	}
	return true
}

func measurementsCSV(measurements []domain.Measurement) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)

	header := []string{"measured_at", "weight_kg", "body_fat_pct", "neck_cm", "waist_cm", "hips_cm"}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	// Chronological order is more useful in a spreadsheet than the newest-first
	// order used by the screens.
	for i := len(measurements) - 1; i >= 0; i-- {
		measurement := measurements[i]
		row := []string{
			measurement.MeasuredAt.UTC().Format(time.RFC3339),
			formatCSVFloat(measurement.WeightKG),
			formatCSVFloatPtr(measurement.BodyFatPct),
			formatCSVFloatPtr(measurement.NeckCM),
			formatCSVFloatPtr(measurement.WaistCM),
			formatCSVFloatPtr(measurement.HipsCM),
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return buffer.Bytes(), nil
}

func formatCSVFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatCSVFloatPtr(value *float64) string {
	if value == nil {
		return ""
	}
	return formatCSVFloat(*value)
}
