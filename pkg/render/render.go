// Package render provides HTML rendering for GoPage components.
package render

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/hazyhaar/gopage/pkg/engine"
)

// Component renders a single result to HTML.
type Component interface {
	// Name returns the component name (e.g., "table", "form").
	Name() string

	// Render writes the component HTML to the writer.
	Render(w io.Writer, result *engine.Result, data *PageData) error
}

// PageData contains data available to all templates.
type PageData struct {
	Title       string
	Results     []*engine.Result
	CurrentPath string
	IsHTMX      bool
	Error       error
}

// Renderer manages component rendering.
type Renderer struct {
	templates  *template.Template
	components map[string]Component
	logger     *slog.Logger
}

// Config holds renderer configuration.
type Config struct {
	TemplatesFS fs.FS
	Logger      *slog.Logger
}

// New creates a new renderer.
func New(cfg Config) (*Renderer, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Parse all templates
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(cfg.TemplatesFS, "**/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	r := &Renderer{
		templates:  tmpl,
		components: make(map[string]Component),
		logger:     cfg.Logger,
	}

	// Register built-in components
	r.Register(&TextComponent{tmpl: tmpl})
	r.Register(&TableComponent{tmpl: tmpl})
	r.Register(&ListComponent{tmpl: tmpl})
	r.Register(&CardComponent{tmpl: tmpl})
	r.Register(&ShellComponent{tmpl: tmpl})
	r.Register(&FormComponent{tmpl: tmpl})
	r.Register(&ErrorComponent{tmpl: tmpl})

	return r, nil
}

// Register adds a component to the renderer.
func (r *Renderer) Register(c Component) {
	r.components[c.Name()] = c
}

// RenderPage renders a full page with all results.
func (r *Renderer) RenderPage(w io.Writer, data *PageData) error {
	var content bytes.Buffer

	// Render each result with its component
	for _, result := range data.Results {
		component, ok := r.components[result.Query.Component]
		if !ok {
			r.logger.Warn("unknown component", "name", result.Query.Component)
			component = r.components["text"]
		}

		if err := component.Render(&content, result, data); err != nil {
			return fmt.Errorf("render %s: %w", result.Query.Component, err)
		}
	}

	// If HTMX request, return only content
	if data.IsHTMX {
		_, err := w.Write(content.Bytes())
		return err
	}

	// Wrap in layout
	layoutData := struct {
		*PageData
		Content template.HTML
	}{
		PageData: data,
		Content:  template.HTML(content.String()),
	}

	return r.templates.ExecuteTemplate(w, "layouts/base.html", layoutData)
}

// RenderError renders an error page.
func (r *Renderer) RenderError(w io.Writer, data *PageData) error {
	errComponent := r.components["error"]

	var content bytes.Buffer
	if err := errComponent.Render(&content, nil, data); err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	if data.IsHTMX {
		_, err := w.Write(content.Bytes())
		return err
	}

	layoutData := struct {
		*PageData
		Content template.HTML
	}{
		PageData: data,
		Content:  template.HTML(content.String()),
	}

	return r.templates.ExecuteTemplate(w, "layouts/base.html", layoutData)
}

// templateFuncs provides helper functions for templates.
var templateFuncs = template.FuncMap{
	"safe": func(s string) template.HTML {
		return template.HTML(s)
	},
	"json": func(v interface{}) string {
		return fmt.Sprintf("%v", v)
	},
	"lower": strings.ToLower,
	"upper": strings.ToUpper,
	"title": strings.Title,
	"default": func(def, val interface{}) interface{} {
		if val == nil || val == "" {
			return def
		}
		return val
	},
	"dict": func(pairs ...interface{}) map[string]interface{} {
		m := make(map[string]interface{})
		for i := 0; i < len(pairs)-1; i += 2 {
			key, _ := pairs[i].(string)
			m[key] = pairs[i+1]
		}
		return m
	},
	"hasSubmit": func(rows []map[string]interface{}) bool {
		for _, row := range rows {
			if t, ok := row["type"].(string); ok && t == "submit" {
				return true
			}
		}
		return false
	},
}

// MustLoadTemplates is a helper to load embedded templates.
func MustLoadTemplates(fsys embed.FS) fs.FS {
	sub, err := fs.Sub(fsys, "templates")
	if err != nil {
		panic(err)
	}
	return sub
}
