package render

import (
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Renderer struct {
	pages    map[string]*template.Template
	partials *template.Template
}

func ResolveProjectPath(parts ...string) string {
	candidates := make([]string, 0, 8)

	// 1. Check relative to executable location (highest priority for production releases)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// Check ../web relative to executable (e.g. bin/wedrink -> ../web)
		candidates = append(candidates, filepath.Join(append([]string{exeDir, ".."}, parts...)...))
		// Check web in same dir as executable
		candidates = append(candidates, filepath.Join(append([]string{exeDir}, parts...)...))
	}

	// 2. Check current working directory (for local dev with go run / air)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(append([]string{cwd}, parts...)...))
	}

	for _, candidate := range candidates {
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	return filepath.Join(parts...)
}

func NewRenderer(funcMap template.FuncMap) (*Renderer, error) {
	layoutFile := ResolveProjectPath("web", "templates", "layout.html")
	compFiles, _ := filepath.Glob(filepath.Join(filepath.Dir(layoutFile), "components", "*.html"))

	// Explicit page mappings with their required sub-templates to prevent block collision
	pageConfig := map[string][]string{
		"login.html": {
			ResolveProjectPath("web", "templates", "login.html"),
		},
		"dashboard.html": {
			ResolveProjectPath("web", "templates", "dashboard.html"),
			ResolveProjectPath("web", "templates", "dashboard_content.html"),
		},
		"submit.html": {
			ResolveProjectPath("web", "templates", "submit.html"),
		},
		"reports.html": {
			ResolveProjectPath("web", "templates", "reports.html"),
			ResolveProjectPath("web", "templates", "report_table.html"),
		},
		"admin_users.html": {
			ResolveProjectPath("web", "templates", "admin_users.html"),
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
	templatesDir := filepath.Dir(layoutFile)
	allTemplates, _ := filepath.Glob(filepath.Join(templatesDir, "*.html"))
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
