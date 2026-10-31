package monitoring

import "testing"

func TestSanitizeNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "hyphen", in: "mega-trainer", want: "mega_trainer"},
		{name: "spaces and dots", in: "mega trainer.v2", want: "mega_trainer_v2"},
		{name: "leading digit", in: "1bot", want: "_bot"},
		{name: "already valid", in: "mega_trainer", want: "mega_trainer"},
		{name: "empty", in: "", want: "mega_trainer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sanitizeNamespace(tt.in); got != tt.want {
				t.Fatalf("sanitizeNamespace(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNewMetricsAcceptsDashedName(t *testing.T) {
	// The default config uses "mega-trainer"; metric names must stay scrapeable.
	metrics := NewMetrics("mega-trainer")
	if metrics == nil {
		t.Fatal("metrics is nil")
	}
}
