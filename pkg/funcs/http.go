package funcs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"zombiezen.com/go/sqlite"
)

// Default HTTP client with timeout
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// HTTPFuncs returns HTTP request functions.
func HTTPFuncs() []Func {
	return []Func{
		{
			Name:          "http_get",
			NumArgs:       1, // url
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				urlStr := args[0].Text()

				resp, err := httpClient.Get(urlStr)
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
				if err != nil {
					return sqlite.TextValue(""), nil
				}

				return sqlite.TextValue(string(body)), nil
			},
		},
		{
			Name:          "http_get_json",
			NumArgs:       2, // url, json_path (optional, empty for full response)
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				urlStr := args[0].Text()
				path := args[1].Text()

				resp, err := httpClient.Get(urlStr)
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if err != nil {
					return sqlite.TextValue(""), nil
				}

				// If no path, return full JSON
				if path == "" {
					return sqlite.TextValue(string(body)), nil
				}

				// Parse and extract path
				var data interface{}
				if err := json.Unmarshal(body, &data); err != nil {
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

				switch v := current.(type) {
				case string:
					return sqlite.TextValue(v), nil
				case float64:
					return sqlite.FloatValue(v), nil
				default:
					b, _ := json.Marshal(v)
					return sqlite.TextValue(string(b)), nil
				}
			},
		},
		{
			Name:          "http_post",
			NumArgs:       3, // url, content_type, body
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				urlStr := args[0].Text()
				contentType := args[1].Text()
				bodyStr := args[2].Text()

				resp, err := httpClient.Post(urlStr, contentType, strings.NewReader(bodyStr))
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if err != nil {
					return sqlite.TextValue(""), nil
				}

				return sqlite.TextValue(string(body)), nil
			},
		},
		{
			Name:          "http_post_json",
			NumArgs:       2, // url, json_body
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				urlStr := args[0].Text()
				jsonBody := args[1].Text()

				resp, err := httpClient.Post(urlStr, "application/json", strings.NewReader(jsonBody))
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if err != nil {
					return sqlite.TextValue(""), nil
				}

				return sqlite.TextValue(string(body)), nil
			},
		},
		{
			Name:          "http_request",
			NumArgs:       4, // method, url, headers_json, body
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				method := strings.ToUpper(args[0].Text())
				urlStr := args[1].Text()
				headersJSON := args[2].Text()
				bodyStr := args[3].Text()

				var bodyReader io.Reader
				if bodyStr != "" {
					bodyReader = strings.NewReader(bodyStr)
				}

				req, err := http.NewRequestWithContext(context.Background(), method, urlStr, bodyReader)
				if err != nil {
					return sqlite.TextValue(""), nil
				}

				// Parse headers
				if headersJSON != "" {
					var headers map[string]string
					if err := json.Unmarshal([]byte(headersJSON), &headers); err == nil {
						for k, v := range headers {
							req.Header.Set(k, v)
						}
					}
				}

				resp, err := httpClient.Do(req)
				if err != nil {
					return sqlite.TextValue(""), nil
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if err != nil {
					return sqlite.TextValue(""), nil
				}

				// Return response with status
				result := map[string]interface{}{
					"status":  resp.StatusCode,
					"body":    string(body),
					"headers": resp.Header,
				}
				b, _ := json.Marshal(result)
				return sqlite.TextValue(string(b)), nil
			},
		},
		{
			Name:          "url_encode",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				return sqlite.TextValue(url.QueryEscape(s)), nil
			},
		},
		{
			Name:          "url_decode",
			NumArgs:       1,
			Deterministic: true,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				s := args[0].Text()
				decoded, err := url.QueryUnescape(s)
				if err != nil {
					return sqlite.TextValue(s), nil
				}
				return sqlite.TextValue(decoded), nil
			},
		},
	}
}

// SetHTTPTimeout sets the HTTP client timeout.
func SetHTTPTimeout(d time.Duration) {
	httpClient.Timeout = d
}
