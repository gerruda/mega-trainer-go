package plot

import (
	"bytes"
	"errors"
	"testing"
)

func TestRenderLineChartReturnsPNG(t *testing.T) {
	t.Parallel()

	png, err := RenderLineChart([]float64{70.5, 71.2, 69.8})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if len(png) == 0 {
		t.Fatal("rendered image is empty")
	}

	pngSignature := []byte{0x89, 'P', 'N', 'G'}
	if !bytes.HasPrefix(png, pngSignature) {
		t.Fatalf("image does not start with a PNG signature: %x", png[:4])
	}
}

func TestRenderLineChartRequiresData(t *testing.T) {
	t.Parallel()

	if _, err := RenderLineChart(nil); !errors.Is(err, ErrNoData) {
		t.Fatalf("error = %v, want ErrNoData", err)
	}
}
