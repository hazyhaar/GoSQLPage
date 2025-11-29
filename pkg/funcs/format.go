package funcs

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
)

// FormatFuncs returns formatting functions.
func FormatFuncs() []Func {
	return []Func{
		{
			Name:          "format_number",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				// Format number with thousand separators
				n := args[0].Float()
				return sqlite.TextValue(formatNumberWithCommas(n)), nil
			},
		},
		{
			Name:          "format_number_decimals",
			NumArgs:       2, // number, decimals
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				n := args[0].Float()
				decimals := int(args[1].Int64())
				formatted := strconv.FormatFloat(n, 'f', decimals, 64)
				return sqlite.TextValue(formatNumberWithCommas(parseFloat(formatted))), nil
			},
		},
		{
			Name:          "format_bytes",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				bytes := args[0].Int64()
				return sqlite.TextValue(formatBytes(bytes)), nil
			},
		},
		{
			Name:          "format_percent",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				n := args[0].Float()
				return sqlite.TextValue(fmt.Sprintf("%.1f%%", n*100)), nil
			},
		},
		{
			Name:          "format_currency",
			NumArgs:       2, // amount, currency_symbol
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				amount := args[0].Float()
				symbol := args[1].Text()
				if symbol == "" {
					symbol = "$"
				}
				formatted := formatNumberWithCommas(math.Round(amount*100) / 100)
				// Ensure 2 decimal places
				if !strings.Contains(formatted, ".") {
					formatted += ".00"
				} else {
					parts := strings.Split(formatted, ".")
					if len(parts[1]) == 1 {
						formatted += "0"
					}
				}
				return sqlite.TextValue(symbol + formatted), nil
			},
		},
		{
			Name:          "time_ago",
			NumArgs:       1, // unix timestamp or ISO date string
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				var t time.Time

				// Try to parse as unix timestamp first
				if ts := args[0].Int64(); ts > 0 {
					t = time.Unix(ts, 0)
				} else {
					// Try to parse as ISO date string
					dateStr := args[0].Text()
					var err error
					t, err = time.Parse(time.RFC3339, dateStr)
					if err != nil {
						t, err = time.Parse("2006-01-02 15:04:05", dateStr)
						if err != nil {
							t, err = time.Parse("2006-01-02", dateStr)
							if err != nil {
								return sqlite.TextValue(""), nil
							}
						}
					}
				}

				return sqlite.TextValue(timeAgo(t)), nil
			},
		},
		{
			Name:          "time_until",
			NumArgs:       1, // unix timestamp or ISO date string
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				var t time.Time

				if ts := args[0].Int64(); ts > 0 {
					t = time.Unix(ts, 0)
				} else {
					dateStr := args[0].Text()
					var err error
					t, err = time.Parse(time.RFC3339, dateStr)
					if err != nil {
						t, err = time.Parse("2006-01-02 15:04:05", dateStr)
						if err != nil {
							return sqlite.TextValue(""), nil
						}
					}
				}

				return sqlite.TextValue(timeUntil(t)), nil
			},
		},
		{
			Name:          "format_duration",
			NumArgs:       1, // seconds
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				seconds := args[0].Int64()
				return sqlite.TextValue(formatDuration(seconds)), nil
			},
		},
		{
			Name:          "pluralize",
			NumArgs:       3, // count, singular, plural
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				count := args[0].Int64()
				singular := args[1].Text()
				plural := args[2].Text()
				if count == 1 {
					return sqlite.TextValue(fmt.Sprintf("%d %s", count, singular)), nil
				}
				return sqlite.TextValue(fmt.Sprintf("%d %s", count, plural)), nil
			},
		},
		{
			Name:          "ordinal",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				n := args[0].Int64()
				return sqlite.TextValue(ordinal(n)), nil
			},
		},
	}
}

// formatNumberWithCommas formats a number with thousand separators.
func formatNumberWithCommas(n float64) string {
	// Handle negative numbers
	neg := n < 0
	if neg {
		n = -n
	}

	// Split into integer and decimal parts
	intPart := int64(n)
	decPart := n - float64(intPart)

	// Format integer part with commas
	str := strconv.FormatInt(intPart, 10)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}

	// Add decimal part if present
	if decPart > 0.0001 {
		decStr := strconv.FormatFloat(decPart, 'f', -1, 64)
		if len(decStr) > 1 {
			result += decStr[1:] // Skip the leading "0"
		}
	}

	if neg {
		result = "-" + result
	}
	return result
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// timeAgo returns a human-readable string of time elapsed.
func timeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		return "in the future"
	}

	seconds := int64(diff.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24
	weeks := days / 7
	months := days / 30
	years := days / 365

	switch {
	case seconds < 60:
		return "just now"
	case minutes == 1:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours == 1:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "yesterday"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case weeks == 1:
		return "1 week ago"
	case weeks < 4:
		return fmt.Sprintf("%d weeks ago", weeks)
	case months == 1:
		return "1 month ago"
	case months < 12:
		return fmt.Sprintf("%d months ago", months)
	case years == 1:
		return "1 year ago"
	default:
		return fmt.Sprintf("%d years ago", years)
	}
}

// timeUntil returns a human-readable string of time remaining.
func timeUntil(t time.Time) string {
	now := time.Now()
	diff := t.Sub(now)

	if diff < 0 {
		return "already passed"
	}

	seconds := int64(diff.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	switch {
	case seconds < 60:
		return "in a moment"
	case minutes == 1:
		return "in 1 minute"
	case minutes < 60:
		return fmt.Sprintf("in %d minutes", minutes)
	case hours == 1:
		return "in 1 hour"
	case hours < 24:
		return fmt.Sprintf("in %d hours", hours)
	case days == 1:
		return "tomorrow"
	case days < 7:
		return fmt.Sprintf("in %d days", days)
	default:
		return fmt.Sprintf("in %d days", days)
	}
}

// formatDuration formats seconds into a readable duration.
func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		m := seconds / 60
		s := seconds % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm %ds", m, s)
	}
	if seconds < 86400 {
		h := seconds / 3600
		m := (seconds % 3600) / 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	d := seconds / 86400
	h := (seconds % 86400) / 3600
	if h == 0 {
		return fmt.Sprintf("%dd", d)
	}
	return fmt.Sprintf("%dd %dh", d, h)
}

// ordinal returns the ordinal representation of a number.
func ordinal(n int64) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
