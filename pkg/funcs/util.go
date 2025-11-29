package funcs

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"zombiezen.com/go/sqlite"
)

// UtilFuncs returns utility functions.
func UtilFuncs() []Func {
	return []Func{
		{
			Name:          "uuid",
			NumArgs:       0,
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				// Generate UUIDv4
				b := make([]byte, 16)
				rand.Read(b)
				b[6] = (b[6] & 0x0f) | 0x40 // Version 4
				b[8] = (b[8] & 0x3f) | 0x80 // Variant
				uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
					b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
				return sqlite.TextValue(uuid), nil
			},
		},
		{
			Name:          "uuid_short",
			NumArgs:       0,
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				// Generate short ID (8 chars)
				b := make([]byte, 4)
				rand.Read(b)
				return sqlite.TextValue(fmt.Sprintf("%08x", b)), nil
			},
		},
		{
			Name:          "random_int",
			NumArgs:       2, // min, max
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				min := args[0].Int64()
				max := args[1].Int64()
				if max <= min {
					return sqlite.IntegerValue(min), nil
				}
				n, _ := rand.Int(rand.Reader, big.NewInt(max-min))
				return sqlite.IntegerValue(min + n.Int64()), nil
			},
		},
		{
			Name:          "random_string",
			NumArgs:       1, // length
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				length := int(args[0].Int64())
				if length <= 0 || length > 10000 {
					length = 10
				}
				const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
				result := make([]byte, length)
				for i := range result {
					n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
					result[i] = chars[n.Int64()]
				}
				return sqlite.TextValue(string(result)), nil
			},
		},
		{
			Name:          "now_unix",
			NumArgs:       0,
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				return sqlite.IntegerValue(time.Now().Unix()), nil
			},
		},
		{
			Name:          "now_unix_ms",
			NumArgs:       0,
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				return sqlite.IntegerValue(time.Now().UnixMilli()), nil
			},
		},
		{
			Name:          "now_iso",
			NumArgs:       0,
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				return sqlite.TextValue(time.Now().UTC().Format(time.RFC3339)), nil
			},
		},
		{
			Name:          "format_date",
			NumArgs:       2, // unix_timestamp, format
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				ts := args[0].Int64()
				format := args[1].Text()
				t := time.Unix(ts, 0)
				// Convert common format strings to Go format
				format = convertDateFormat(format)
				return sqlite.TextValue(t.Format(format)), nil
			},
		},
		{
			Name:          "parse_date",
			NumArgs:       2, // date_string, format
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				dateStr := args[0].Text()
				format := args[1].Text()
				format = convertDateFormat(format)
				t, err := time.Parse(format, dateStr)
				if err != nil {
					return sqlite.IntegerValue(0), nil
				}
				return sqlite.IntegerValue(t.Unix()), nil
			},
		},
		{
			Name:          "env",
			NumArgs:       1, // variable name
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				name := args[0].Text()
				return sqlite.TextValue(os.Getenv(name)), nil
			},
		},
		{
			Name:          "env_or",
			NumArgs:       2, // variable name, default
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				name := args[0].Text()
				def := args[1].Text()
				if v := os.Getenv(name); v != "" {
					return sqlite.TextValue(v), nil
				}
				return sqlite.TextValue(def), nil
			},
		},
		{
			Name:          "coalesce_empty",
			NumArgs:       -1, // variadic
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				for _, arg := range args {
					if s := arg.Text(); s != "" {
						return sqlite.TextValue(s), nil
					}
				}
				return sqlite.TextValue(""), nil
			},
		},
		{
			Name:          "if_then",
			NumArgs:       3, // condition, then_value, else_value
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				cond := args[0].Int64()
				thenVal := args[1].Text()
				elseVal := args[2].Text()
				if cond != 0 {
					return sqlite.TextValue(thenVal), nil
				}
				return sqlite.TextValue(elseVal), nil
			},
		},
		{
			Name:          "to_int",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				i, _ := strconv.ParseInt(s, 10, 64)
				return sqlite.IntegerValue(i), nil
			},
		},
		{
			Name:          "to_float",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				f, _ := strconv.ParseFloat(s, 64)
				return sqlite.FloatValue(f), nil
			},
		},
	}
}

// convertDateFormat converts common date format strings to Go format.
func convertDateFormat(format string) string {
	replacements := map[string]string{
		"YYYY": "2006",
		"YY":   "06",
		"MM":   "01",
		"DD":   "02",
		"HH":   "15",
		"mm":   "04",
		"ss":   "05",
		"SSS":  "000",
	}
	for k, v := range replacements {
		format = replaceAll(format, k, v)
	}
	return format
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
