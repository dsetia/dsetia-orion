package common

import "time"

// Liveness thresholds are based on poll_interval_mins (default 10 min):
//
//	green  — last seen < 20 min ago  (within 2 poll intervals — nominal)
//	yellow — last seen < 30 min ago  (within 3 poll intervals — degraded)
//	red    — last seen ≥ 30 min ago, or sensor has never polled

const (
	LivenessGreen  = "green"
	LivenessYellow = "yellow"
	LivenessRed    = "red"
)

// DeviceLiveness returns "green", "yellow", or "red" based on how recently
// a sensor last polled (as recorded in version.updated_at).
// Pass nil when the sensor has never polled; that returns "red".
func DeviceLiveness(lastSeen *time.Time) string {
	if lastSeen == nil {
		return LivenessRed
	}
	age := time.Since(*lastSeen)
	switch {
	case age < 20*time.Minute:
		return LivenessGreen
	case age < 30*time.Minute:
		return LivenessYellow
	default:
		return LivenessRed
	}
}
