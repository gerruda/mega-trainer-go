// Package plot renders user data as PNG charts that can be sent to a chat.
package plot

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/wcharczuk/go-chart/v2"
)

// ErrNoData is returned when there is nothing to plot.
var ErrNoData = errors.New("plot: no data points")

// RenderLineChart renders a line chart with one point per value and returns the
// PNG bytes. The values are indexed on the X axis starting from 1.
func RenderLineChart(values []float64) ([]byte, error) {
	if len(values) == 0 {
		return nil, ErrNoData
	}

	xValues := make([]float64, len(values))
	for i := range values {
		xValues[i] = float64(i + 1)
	}

	graph := chart.Chart{
		Title: "Динамика",
		YAxis: chart.YAxis{
			Name: "Значение",
			ValueFormatter: func(v any) string {
				if f, ok := v.(float64); ok {
					return fmt.Sprintf("%.1f", f)
				}
				return fmt.Sprintf("%v", v)
			},
		},
		XAxis: chart.XAxis{Name: "Измерение"},
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    "Результат",
				XValues: xValues,
				YValues: values,
				Style: chart.Style{
					StrokeColor: chart.ColorBlue,
					StrokeWidth: 2,
				},
			},
		},
	}

	buf := &bytes.Buffer{}
	if err := graph.Render(chart.PNG, buf); err != nil {
		return nil, fmt.Errorf("render chart: %w", err)
	}

	return buf.Bytes(), nil
}
