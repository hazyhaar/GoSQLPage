package funcs

import (
	"html"
	"regexp"
	"strings"

	"zombiezen.com/go/sqlite"
)

// StringFuncs returns string manipulation functions.
func StringFuncs() []Func {
	return []Func{
		{
			Name:          "str_reverse",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				runes := []rune(s)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				return sqlite.TextValue(string(runes)), nil
			},
		},
		{
			Name:          "str_repeat",
			NumArgs:       2,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				n := int(args[1].Int64())
				if n < 0 {
					n = 0
				}
				if n > 10000 {
					n = 10000 // safety limit
				}
				return sqlite.TextValue(strings.Repeat(s, n)), nil
			},
		},
		{
			Name:          "str_slug",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				// Simple slug: lowercase, replace spaces with dashes, remove special chars
				s = strings.ToLower(s)
				s = strings.ReplaceAll(s, " ", "-")
				reg := regexp.MustCompile(`[^a-z0-9-]`)
				s = reg.ReplaceAllString(s, "")
				reg = regexp.MustCompile(`-+`)
				s = reg.ReplaceAllString(s, "-")
				s = strings.Trim(s, "-")
				return sqlite.TextValue(s), nil
			},
		},
		{
			Name:          "str_truncate",
			NumArgs:       2,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				maxLen := int(args[1].Int64())
				if len(s) <= maxLen {
					return sqlite.TextValue(s), nil
				}
				if maxLen <= 3 {
					return sqlite.TextValue(s[:maxLen]), nil
				}
				return sqlite.TextValue(s[:maxLen-3] + "..."), nil
			},
		},
		{
			Name:          "str_contains",
			NumArgs:       2,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				haystack := args[0].Text()
				needle := args[1].Text()
				if strings.Contains(haystack, needle) {
					return sqlite.IntegerValue(1), nil
				}
				return sqlite.IntegerValue(0), nil
			},
		},
		{
			Name:          "str_split",
			NumArgs:       3, // string, delimiter, index
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				delim := args[1].Text()
				idx := int(args[2].Int64())
				parts := strings.Split(s, delim)
				if idx < 0 || idx >= len(parts) {
					return sqlite.TextValue(""), nil
				}
				return sqlite.TextValue(parts[idx]), nil
			},
		},
		{
			Name:          "escape_html",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				return sqlite.TextValue(html.EscapeString(s)), nil
			},
		},
		{
			Name:          "unescape_html",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				return sqlite.TextValue(html.UnescapeString(s)), nil
			},
		},
	}
}
