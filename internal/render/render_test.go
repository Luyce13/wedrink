package render

import (
	"os"
	"path/filepath"
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
