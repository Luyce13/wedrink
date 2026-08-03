package render

import (
	"html/template"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRenderer(t *testing.T) {
	_ = os.Chdir(filepath.Join("..", ".."))
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"fmtNum": func(val any) string { return "0" },
		"not": func(v bool) bool { return !v },
		"mod": func(a, b int) int { return a % b },
	}

	renderer, err := NewRenderer(funcMap)
	if err != nil {
		t.Fatalf("NewRenderer failed: %v", err)
	}

	if renderer == nil {
		t.Fatal("expected non-nil renderer")
	}
}
