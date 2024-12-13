package helper

import (
	"fmt"
	"time"
)

func pluralize(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
}

func GetTimeAgo(past time.Time) string {
	now := time.Now()
	duration := now.Sub(past)

	hours := int(duration.Hours())
	// minutes := int(duration.Minutes()) % 60
	// seconds := int(duration.Seconds()) % 60

	if hours > 732 { // average of 720 = 30 days and 744 = 31 days month
		months := hours / 732
		return fmt.Sprintf("%d month%s ago", months, pluralize(months))
	} else if hours > 24 {
		days := hours / 24
		return fmt.Sprintf("%d day%s ago", days, pluralize(days))
	} else if hours > 0 {
		return fmt.Sprintf("%d hour%s ago", hours, pluralize(hours))
		// } else if minutes > 0 {
		// return fmt.Sprintf("%d minute%s ago", minutes, pluralize(minutes))
		// } else {
		// 	return fmt.Sprintf("%d second%s ago", seconds, pluralize(seconds))
	}

	return "today"
}
