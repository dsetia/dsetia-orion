package common

import (
	"testing"
	"time"
)

func TestDeviceLiveness(t *testing.T) {
	ago := func(d time.Duration) *time.Time {
		t := time.Now().Add(-d)
		return &t
	}

	tests := []struct {
		name     string
		lastSeen *time.Time
		want     string
	}{
		{"nil (never polled)", nil, LivenessRed},
		{"just now", ago(0), LivenessGreen},
		{"10 min ago", ago(10 * time.Minute), LivenessGreen},
		{"19 min ago", ago(19 * time.Minute), LivenessGreen},
		{"boundary: exactly 20 min", ago(20 * time.Minute), LivenessYellow},
		{"25 min ago", ago(25 * time.Minute), LivenessYellow},
		{"boundary: exactly 30 min", ago(30 * time.Minute), LivenessRed},
		{"1 hour ago", ago(60 * time.Minute), LivenessRed},
		{"24 hours ago", ago(24 * time.Hour), LivenessRed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeviceLiveness(tc.lastSeen)
			if got != tc.want {
				t.Errorf("DeviceLiveness = %q, want %q", got, tc.want)
			}
		})
	}
}
