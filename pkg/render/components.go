package render

import (
	"html/template"
	"io"

	"github.com/hazyhaar/gopage/pkg/engine"
)

// TextComponent renders simple text content.
type TextComponent struct {
	tmpl *template.Template
}

func (c *TextComponent) Name() string { return "text" }

func (c *TextComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/text.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// TableComponent renders data as an HTML table.
type TableComponent struct {
	tmpl *template.Template
}

func (c *TableComponent) Name() string { return "table" }

func (c *TableComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/table.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// ListComponent renders data as a list.
type ListComponent struct {
	tmpl *template.Template
}

func (c *ListComponent) Name() string { return "list" }

func (c *ListComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/list.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// CardComponent renders data as cards.
type CardComponent struct {
	tmpl *template.Template
}

func (c *CardComponent) Name() string { return "card" }

func (c *CardComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/card.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// ShellComponent renders the page shell (navbar, footer).
type ShellComponent struct {
	tmpl *template.Template
}

func (c *ShellComponent) Name() string { return "shell" }

func (c *ShellComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	// Shell sets page-level options, doesn't render directly
	if result != nil && result.Query.Options != nil {
		if title, ok := result.Query.Options["title"]; ok {
			data.Title = title
		}
	}
	return nil
}

// FormComponent renders an HTML form.
type FormComponent struct {
	tmpl *template.Template
}

func (c *FormComponent) Name() string { return "form" }

func (c *FormComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/form.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// ErrorComponent renders error messages.
type ErrorComponent struct {
	tmpl *template.Template
}

func (c *ErrorComponent) Name() string { return "error" }

func (c *ErrorComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "system/error.html", data)
}

// SearchComponent renders a search form.
type SearchComponent struct {
	tmpl *template.Template
}

func (c *SearchComponent) Name() string { return "search" }

func (c *SearchComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/search.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// AlertComponent renders alert messages (success, error, warning, info).
type AlertComponent struct {
	tmpl *template.Template
}

func (c *AlertComponent) Name() string { return "alert" }

func (c *AlertComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/alert.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}

// SSEComponent renders a Server-Sent Events subscriber.
type SSEComponent struct {
	tmpl *template.Template
}

func (c *SSEComponent) Name() string { return "sse" }

func (c *SSEComponent) Render(w io.Writer, result *engine.Result, data *PageData) error {
	return c.tmpl.ExecuteTemplate(w, "components/sse.html", struct {
		Result  *engine.Result
		Options map[string]string
	}{
		Result:  result,
		Options: result.Query.Options,
	})
}
