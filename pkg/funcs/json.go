package funcs

import (
	"encoding/json"
	"strings"

	"zombiezen.com/go/sqlite"
)

// JSONFuncs returns JSON manipulation functions.
func JSONFuncs() []Func {
	return []Func{
		{
			Name:          "json_get",
			NumArgs:       2, // json_string, path (dot notation)
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				jsonStr := args[0].Text()
				path := args[1].Text()

				var data interface{}
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return sqlite.TextValue(""), nil
				}

				// Navigate path
				parts := strings.Split(path, ".")
				current := data
				for _, part := range parts {
					if part == "" {
						continue
					}
					switch v := current.(type) {
					case map[string]interface{}:
						current = v[part]
					default:
						return sqlite.TextValue(""), nil
					}
				}

				// Return result based on type
				switch v := current.(type) {
				case string:
					return sqlite.TextValue(v), nil
				case float64:
					return sqlite.FloatValue(v), nil
				case bool:
					if v {
						return sqlite.IntegerValue(1), nil
					}
					return sqlite.IntegerValue(0), nil
				case nil:
					return sqlite.TextValue(""), nil
				default:
					// Return as JSON string
					b, _ := json.Marshal(v)
					return sqlite.TextValue(string(b)), nil
				}
			},
		},
		{
			Name:          "json_set",
			NumArgs:       3, // json_string, path, value
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				jsonStr := args[0].Text()
				path := args[1].Text()
				value := args[2].Text()

				var data map[string]interface{}
				if jsonStr == "" {
					data = make(map[string]interface{})
				} else if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					data = make(map[string]interface{})
				}

				// Parse value (try as JSON first, then as string)
				var val interface{} = value
				var jsonVal interface{}
				if err := json.Unmarshal([]byte(value), &jsonVal); err == nil {
					val = jsonVal
				}

				// Set value at path
				parts := strings.Split(path, ".")
				current := data
				for i, part := range parts {
					if part == "" {
						continue
					}
					if i == len(parts)-1 {
						current[part] = val
					} else {
						if _, ok := current[part]; !ok {
							current[part] = make(map[string]interface{})
						}
						if m, ok := current[part].(map[string]interface{}); ok {
							current = m
						}
					}
				}

				result, _ := json.Marshal(data)
				return sqlite.TextValue(string(result)), nil
			},
		},
		{
			Name:          "json_keys",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				jsonStr := args[0].Text()

				var data map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return sqlite.TextValue("[]"), nil
				}

				keys := make([]string, 0, len(data))
				for k := range data {
					keys = append(keys, k)
				}

				result, _ := json.Marshal(keys)
				return sqlite.TextValue(string(result)), nil
			},
		},
		{
			Name:          "json_array_length",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				jsonStr := args[0].Text()

				var data []interface{}
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return sqlite.IntegerValue(0), nil
				}

				return sqlite.IntegerValue(int64(len(data))), nil
			},
		},
		{
			Name:          "json_pretty",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				jsonStr := args[0].Text()

				var data interface{}
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return sqlite.TextValue(jsonStr), nil
				}

				result, _ := json.MarshalIndent(data, "", "  ")
				return sqlite.TextValue(string(result)), nil
			},
		},
	}
}
