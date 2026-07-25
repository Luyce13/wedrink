package render

import (
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"path/filepath"
)

type Renderer struct {
	pages    map[string]*template.Template
	partials *template.Template
}

func NewRenderer(funcMap template.FuncMap) (*Renderer, error) {
	layoutFile := filepath.Join("web", "templates", "layout.html")
	compFiles, _ := filepath.Glob(filepath.Join("web", "templates", "components", "*.html"))

	// Explicit page mappings with their required sub-templates to prevent block collision
	pageConfig := map[string][]string{
		"login.html": {
			filepath.Join("web", "templates", "login.html"),
		},
		"dashboard.html": {
			filepath.Join("web", "templates", "dashboard.html"),
			filepath.Join("web", "templates", "dashboard_content.html"),
		},
		"submit.html": {
			filepath.Join("web", "templates", "submit.html"),
		},
		"reports.html": {
			filepath.Join("web", "templates", "reports.html"),
			filepath.Join("web", "templates", "report_table.html"),
		},
	}

	pages := make(map[string]*template.Template)

	for pageName, pageSubFiles := range pageConfig {
		files := []string{layoutFile}
		files = append(files, pageSubFiles...)
		files = append(files, compFiles...)

		t, err := template.New(pageName).Funcs(funcMap).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("failed to parse page template %s: %w", pageName, err)
		}
		pages[pageName] = t
	}

	// Standalone collection for HTMX partial snippets
	allTemplates, _ := filepath.Glob(filepath.Join("web", "templates", "*.html"))
	allFiles := append(allTemplates, compFiles...)
	partials, err := template.New("partials").Funcs(funcMap).ParseFiles(allFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse partial templates: %w", err)
	}

	return &Renderer{
		pages:    pages,
		partials: partials,
	}, nil
}

func (r *Renderer) RenderPage(w io.Writer, pageName string, data any) error {
	t, ok := r.pages[pageName]
	if !ok {
		err := fmt.Errorf("page template %s not found", pageName)
		slog.Error("RenderPage error", "error", err)
		return err
	}
	err := t.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		slog.Error("Template execution failed", "page", pageName, "error", err)
	}
	return err
}

func (r *Renderer) RenderPartial(w io.Writer, partialName string, data any) error {
	err := r.partials.ExecuteTemplate(w, partialName, data)
	if err != nil {
		slog.Error("Partial template execution failed", "partial", partialName, "error", err)
	}
	return err
}
