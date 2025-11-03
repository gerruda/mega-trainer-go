package plot

import (
	"bytes"
	"fmt"

	"github.com/wcharczuk/go-chart/v2"
)

func RenderLineChart(values []float64) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}

	xValues := make([]float64, len(values))
	for i := range values {
		xValues[i] = float64(i + 1)
	}

	// Кастомный форматтер (1 знак после запятой)
	valueFormatter := func(v interface{}) string {
		if vf, ok := v.(float64); ok {
			return fmt.Sprintf("%.1f", vf)
		}
		return fmt.Sprintf("%v", v)
	}

	graph := chart.Chart{
		Title: "Динамика расчётов",
		YAxis: chart.YAxis{
			Name:           "Значение",
			ValueFormatter: valueFormatter,
		},
		XAxis: chart.XAxis{
			Name: "Измерение",
		},
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
	err := graph.Render(chart.PNG, buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
