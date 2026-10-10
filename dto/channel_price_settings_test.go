package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func f64(v float64) *float64 {
	return &v
}

func TestChannelPriceSettingsResolve(t *testing.T) {
	t.Run("nil settings returns identity", func(t *testing.T) {
		var p *ChannelPriceSettings
		got := p.Resolve(timeAt(12, 0))
		if got != IdentityPriceFactors() {
			t.Fatalf("expected identity factors, got %+v", got)
		}
	})

	t.Run("empty settings returns identity", func(t *testing.T) {
		got := (&ChannelPriceSettings{}).Resolve(timeAt(12, 0))
		if got != IdentityPriceFactors() {
			t.Fatalf("expected identity factors, got %+v", got)
		}
		if !got.IsIdentity() {
			t.Fatalf("expected IsIdentity true")
		}
	})

	t.Run("explicit zero means free", func(t *testing.T) {
		got := (&ChannelPriceSettings{Total: f64(0), Input: f64(0)}).Resolve(timeAt(12, 0))
		if got.Total != 0 || got.Input != 0 {
			t.Fatalf("expected zero factors, got %+v", got)
		}
		if got.Completion != 1 {
			t.Fatalf("expected unset completion to be 1, got %v", got.Completion)
		}
		if got.IsIdentity() {
			t.Fatalf("expected IsIdentity false when total is 0")
		}
		if got.Overall() != 0 {
			t.Fatalf("expected overall 0, got %v", got.Overall())
		}
	})

	t.Run("all ratios resolved", func(t *testing.T) {
		p := &ChannelPriceSettings{
			Total:       f64(0.9),
			Input:       f64(0.5),
			Completion:  f64(1.5),
			CacheRead:   f64(0.2),
			CacheWrite:  f64(1.1),
			ImageInput:  f64(2),
			AudioInput:  f64(3),
			AudioOutput: f64(4),
		}
		got := p.Resolve(timeAt(12, 0))
		if got.Total != 0.9 || got.Input != 0.5 || got.Completion != 1.5 ||
			got.CacheRead != 0.2 || got.CacheWrite != 1.1 || got.ImageInput != 2 ||
			got.AudioInput != 3 || got.AudioOutput != 4 {
			t.Fatalf("unexpected factors: %+v", got)
		}
		if got.Time != 1 {
			t.Fatalf("expected time 1 when no windows, got %v", got.Time)
		}
		if expected := 0.9; got.Overall() != expected {
			t.Fatalf("expected overall %v, got %v", expected, got.Overall())
		}
	})

	t.Run("time window hit same day", func(t *testing.T) {
		p := &ChannelPriceSettings{
			TimeWindows: []PriceTimeWindow{
				{TimeWindow: TimeWindow{Start: "12:00", End: "14:00"}, Ratio: 0.8},
			},
		}
		if got := p.Resolve(timeAt(13, 0)); got.Time != 0.8 {
			t.Fatalf("expected time 0.8 inside window, got %v", got.Time)
		}
		if got := p.Resolve(timeAt(15, 0)); got.Time != 1 {
			t.Fatalf("expected time 1 outside window, got %v", got.Time)
		}
		// 边界：start 含、end 不含
		if got := p.Resolve(timeAt(12, 0)); got.Time != 0.8 {
			t.Fatalf("expected time 0.8 at start boundary, got %v", got.Time)
		}
		if got := p.Resolve(timeAt(14, 0)); got.Time != 1 {
			t.Fatalf("expected time 1 at end boundary, got %v", got.Time)
		}
	})

	t.Run("time window cross day", func(t *testing.T) {
		p := &ChannelPriceSettings{
			TimeWindows: []PriceTimeWindow{
				{TimeWindow: TimeWindow{Start: "22:00", End: "08:00"}, Ratio: 0.5},
			},
		}
		if got := p.Resolve(timeAt(23, 0)); got.Time != 0.5 {
			t.Fatalf("expected 0.5 before midnight, got %v", got.Time)
		}
		if got := p.Resolve(timeAt(3, 0)); got.Time != 0.5 {
			t.Fatalf("expected 0.5 after midnight, got %v", got.Time)
		}
		if got := p.Resolve(timeAt(12, 0)); got.Time != 1 {
			t.Fatalf("expected 1 outside cross-day window, got %v", got.Time)
		}
	})

	t.Run("overlapping windows take first match", func(t *testing.T) {
		p := &ChannelPriceSettings{
			TimeWindows: []PriceTimeWindow{
				{TimeWindow: TimeWindow{Start: "10:00", End: "20:00"}, Ratio: 0.7},
				{TimeWindow: TimeWindow{Start: "12:00", End: "14:00"}, Ratio: 0.3},
			},
		}
		got := p.Resolve(timeAt(13, 0))
		if got.Time != 0.7 {
			t.Fatalf("expected first matching window 0.7, got %v", got.Time)
		}
	})

	t.Run("invalid window ignored", func(t *testing.T) {
		p := &ChannelPriceSettings{
			TimeWindows: []PriceTimeWindow{
				{TimeWindow: TimeWindow{Start: "bad", End: "14:00"}, Ratio: 0.1},
				{TimeWindow: TimeWindow{Start: "12:00", End: "12:00"}, Ratio: 0.2},
			},
		}
		if got := p.Resolve(timeAt(13, 0)); got.Time != 1 {
			t.Fatalf("expected invalid windows ignored (time 1), got %v", got.Time)
		}
	})
}

func TestChannelPriceSettingsValidate(t *testing.T) {
	cases := []struct {
		name    string
		settings *ChannelPriceSettings
		wantErr bool
	}{
		{"nil ok", nil, false},
		{"empty ok", &ChannelPriceSettings{}, false},
		{"negative total", &ChannelPriceSettings{Total: f64(-0.1)}, true},
		{"negative input", &ChannelPriceSettings{Input: f64(-1)}, true},
		{"zero ok", &ChannelPriceSettings{Total: f64(0)}, false},
		{"valid windows", &ChannelPriceSettings{TimeWindows: []PriceTimeWindow{{TimeWindow: TimeWindow{Start: "22:00", End: "08:00"}, Ratio: 0.5}}}, false},
		{"bad window start", &ChannelPriceSettings{TimeWindows: []PriceTimeWindow{{TimeWindow: TimeWindow{Start: "25:00", End: "08:00"}, Ratio: 0.5}}}, true},
		{"same start end", &ChannelPriceSettings{TimeWindows: []PriceTimeWindow{{TimeWindow: TimeWindow{Start: "08:00", End: "08:00"}, Ratio: 0.5}}}, true},
		{"negative window ratio", &ChannelPriceSettings{TimeWindows: []PriceTimeWindow{{TimeWindow: TimeWindow{Start: "22:00", End: "08:00"}, Ratio: -0.5}}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.settings.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	t.Run("too many windows", func(t *testing.T) {
		windows := make([]PriceTimeWindow, 0, MaxTimeWindows+1)
		for i := 0; i <= MaxTimeWindows; i++ {
			windows = append(windows, PriceTimeWindow{
				TimeWindow: TimeWindow{Start: "00:00", End: "00:01"},
				Ratio:      1,
			})
		}
		p := &ChannelPriceSettings{TimeWindows: windows}
		if err := p.Validate(); err == nil {
			t.Fatalf("expected error for too many windows")
		}
	})
}

func TestPriceFactorsToMap(t *testing.T) {
	if got := IdentityPriceFactors().ToMap(); len(got) != 0 {
		t.Fatalf("expected empty map for identity, got %v", got)
	}

	factors := IdentityPriceFactors()
	factors.Total = 0.9
	factors.Time = 0.8
	factors.CacheRead = 0.2
	got := factors.ToMap()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %v", got)
	}
	if got["total"] != 0.9 || got["time_window"] != 0.8 || got["cache_read"] != 0.2 {
		t.Fatalf("unexpected map: %v", got)
	}
}

func TestPriceTimeWindowJSONRoundTrip(t *testing.T) {
	raw := `{"start":"22:00","end":"08:00","ratio":0.5}`
	var w PriceTimeWindow
	if err := common.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if w.Start != "22:00" || w.End != "08:00" || w.Ratio != 0.5 {
		t.Fatalf("unexpected parsed window: %+v", w)
	}
}