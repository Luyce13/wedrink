package render

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRenderer(t *testing.T) {
	_ = os.Chdir(filepath.Join("..", ".."))
	renderer, err := NewRenderer(DefaultFuncMap())
	if err != nil {
		t.Fatalf("NewRenderer failed: %v", err)
	}

	if renderer == nil {
		t.Fatal("expected non-nil renderer")
	}
}

func TestToJson(t *testing.T) {
	data := []map[string]any{{"hello": "world", "num": 123}}
	tmpl, err := template.New("test").Funcs(DefaultFuncMap()).Parse(`<script id="test-js" type="application/json">{{toJson .}}</script>`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	rendered := buf.String()
	t.Logf("Rendered: %s", rendered)
	expected := `<script id="test-js" type="application/json">[{"hello":"world","num":123}]</script>`
	if rendered != expected {
		t.Fatalf("Expected %q, got %q", expected, rendered)
	}
}
