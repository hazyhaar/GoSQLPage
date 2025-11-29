package funcs

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"zombiezen.com/go/sqlite"
)

// LLM configuration (set via environment variables)
var (
	llmAPIKey     = os.Getenv("LLM_API_KEY")
	llmAPIURL     = getEnvOrDefault("LLM_API_URL", "https://api.openai.com/v1/chat/completions")
	llmModel      = getEnvOrDefault("LLM_MODEL", "gpt-3.5-turbo")
)

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// LLMFuncs returns LLM-related functions.
func LLMFuncs() []Func {
	return []Func{
		{
			Name:          "llm_complete",
			NumArgs:       1, // prompt
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				prompt := args[0].Text()
				return llmComplete(prompt, llmModel, "")
			},
		},
		{
			Name:          "llm_complete_with_model",
			NumArgs:       2, // prompt, model
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				prompt := args[0].Text()
				model := args[1].Text()
				return llmComplete(prompt, model, "")
			},
		},
		{
			Name:          "llm_complete_with_system",
			NumArgs:       3, // prompt, model, system_prompt
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				prompt := args[0].Text()
				model := args[1].Text()
				system := args[2].Text()
				return llmComplete(prompt, model, system)
			},
		},
		{
			Name:          "llm_json",
			NumArgs:       2, // prompt, json_schema (description of expected JSON)
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				prompt := args[0].Text()
				schema := args[1].Text()
				systemPrompt := "You are a helpful assistant that always responds with valid JSON. " + schema
				return llmComplete(prompt, llmModel, systemPrompt)
			},
		},
		{
			Name:          "llm_summarize",
			NumArgs:       1, // text to summarize
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				text := args[0].Text()
				prompt := "Please summarize the following text concisely:\n\n" + text
				return llmComplete(prompt, llmModel, "You are a helpful assistant that provides concise summaries.")
			},
		},
		{
			Name:          "llm_translate",
			NumArgs:       2, // text, target_language
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				text := args[0].Text()
				targetLang := args[1].Text()
				prompt := "Translate the following text to " + targetLang + ":\n\n" + text
				return llmComplete(prompt, llmModel, "You are a translator. Only output the translation, nothing else.")
			},
		},
		{
			Name:          "llm_extract",
			NumArgs:       2, // text, what_to_extract
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				text := args[0].Text()
				what := args[1].Text()
				prompt := "From the following text, extract " + what + ":\n\n" + text
				return llmComplete(prompt, llmModel, "Extract only the requested information. Be concise.")
			},
		},
		{
			Name:          "llm_classify",
			NumArgs:       2, // text, categories (comma-separated)
			Deterministic: false,
			Func: func(ctx sqlite.Context, args []sqlite.Value) (sqlite.Value, error) {
				text := args[0].Text()
				categories := args[1].Text()
				prompt := "Classify the following text into one of these categories: " + categories + "\n\nText: " + text + "\n\nRespond with only the category name."
				return llmComplete(prompt, llmModel, "You are a classifier. Respond with only the category name, nothing else.")
			},
		},
	}
}

// llmComplete makes an API call to the configured LLM endpoint.
func llmComplete(prompt, model, systemPrompt string) (sqlite.Value, error) {
	if llmAPIKey == "" {
		return sqlite.TextValue("[LLM_API_KEY not set]"), nil
	}

	// Build messages
	messages := []map[string]string{}
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	// Build request body (OpenAI-compatible format)
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", llmAPIURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return sqlite.TextValue("[Request error: " + err.Error() + "]"), nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+llmAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return sqlite.TextValue("[HTTP error: " + err.Error() + "]"), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return sqlite.TextValue("[Read error: " + err.Error() + "]"), nil
	}

	if resp.StatusCode != http.StatusOK {
		return sqlite.TextValue("[API error: " + string(body) + "]"), nil
	}

	// Parse response (OpenAI format)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return sqlite.TextValue("[Parse error: " + err.Error() + "]"), nil
	}

	if result.Error.Message != "" {
		return sqlite.TextValue("[API error: " + result.Error.Message + "]"), nil
	}

	if len(result.Choices) == 0 {
		return sqlite.TextValue(""), nil
	}

	return sqlite.TextValue(strings.TrimSpace(result.Choices[0].Message.Content)), nil
}

// SetLLMConfig sets the LLM configuration.
func SetLLMConfig(apiKey, apiURL, model string) {
	if apiKey != "" {
		llmAPIKey = apiKey
	}
	if apiURL != "" {
		llmAPIURL = apiURL
	}
	if model != "" {
		llmModel = model
	}
}
