package dto

import (
	"testing"
	"time"
)

func TestParseTimeWindows(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		wantLen int
		wantErr bool
	}{
		{"empty string", "", 0, false},
		{"blank string", "  ", 0, false},
		{"single window", `[{"start":"12:00","end":"14:00"}]`, 1, false},
		{"multiple windows", `[{"start":"12:00","end":"14:00"},{"start":"22:00","end":"08:00"}]`, 2, false},
		{"cross-day window", `[{"start":"22:00","end":"08:00"}]`, 1, false},
		{"invalid json", `{"start":"12:00"}`, 0, true},
		{"bad start format", `[{"start":"25:00","end":"14:00"}]`, 0, true},
		{"bad end format", `[{"start":"12:00","end":"14:60"}]`, 0, true},
		{"missing field", `[{"start":"12:00"}]`, 0, true},
		{"same start end", `[{"start":"12:00","end":"12:00"}]`, 0, true},
		{"not array", `"12:00"`, 0, true},
		{"too many windows", `[{"start":"00:00","end":"01:00"},{"start":"01:00","end":"02:00"},{"start":"02:00","end":"03:00"},{"start":"03:00","end":"04:00"},{"start":"04:00","end":"05:00"},{"start":"05:00","end":"06:00"},{"start":"06:00","end":"07:00"},{"start":"07:00","end":"08:00"},{"start":"08:00","end":"09:00"},{"start":"09:00","end":"10:00"},{"start":"10:00","end":"11:00"},{"start":"11:00","end":"12:00"},{"start":"12:00","end":"13:00"},{"start":"13:00","end":"14:00"},{"start":"14:00","end":"15:00"},{"start":"15:00","end":"16:00"},{"start":"16:00","end":"17:00"},{"start":"17:00","end":"18:00"},{"start":"18:00","end":"19:00"},{"start":"19:00","end":"20:00"},{"start":"20:00","end":"21:00"}]`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			windows, err := ParseTimeWindows(c.data)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(windows) != c.wantLen {
				t.Fatalf("expected %d windows, got %d", c.wantLen, len(windows))
			}
		})
	}
}

func timeAt(hour, minute int) time.Time {
	return time.Date(2026, 9, 15, hour, minute, 0, 0, time.Local)
}

func TestIsInTimeWindows(t *testing.T) {
	sameDay := []TimeWindow{{Start: "12:00", End: "14:00"}}
	crossDay := []TimeWindow{{Start: "22:00", End: "08:00"}}
	mixed := []TimeWindow{{Start: "12:00", End: "14:00"}, {Start: "22:00", End: "08:00"}}

	cases := []struct {
		name    string
		windows []TimeWindow
		t       time.Time
		want    bool
	}{
		{"same-day inside", sameDay, timeAt(13, 0), true},
		{"same-day start boundary inclusive", sameDay, timeAt(12, 0), true},
		{"same-day end boundary exclusive", sameDay, timeAt(14, 0), false},
		{"same-day before", sameDay, timeAt(11, 59), false},
		{"same-day after", sameDay, timeAt(14, 1), false},
		{"cross-day late night", crossDay, timeAt(23, 30), true},
		{"cross-day early morning", crossDay, timeAt(7, 59), true},
		{"cross-day end boundary exclusive", crossDay, timeAt(8, 0), false},
		{"cross-day outside noon", crossDay, timeAt(12, 0), false},
		{"cross-day start boundary inclusive", crossDay, timeAt(22, 0), true},
		{"mixed hits second window", mixed, timeAt(23, 0), true},
		{"mixed hits first window", mixed, timeAt(13, 0), true},
		{"mixed outside", mixed, timeAt(16, 0), false},
		{"empty windows", nil, timeAt(13, 0), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsInTimeWindows(c.windows, c.t); got != c.want {
				t.Fatalf("IsInTimeWindows(%v) = %v, want %v", c.t, got, c.want)
			}
		})
	}
}
