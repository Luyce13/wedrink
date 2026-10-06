package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/handlers"
	"wedrink/internal/middleware"
	"wedrink/internal/models"
	"wedrink/internal/render"
	"wedrink/internal/repository"
	"wedrink/internal/services"
)

type mockAuditRepo struct {
	logs  []models.AuditLog
	stats repository.AuditStats
}

func (m *mockAuditRepo) Create(ctx context.Context, log *models.AuditLog) error {
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) Query(ctx context.Context, params repository.AuditQueryParams) ([]models.AuditLog, error) {
	return m.logs, nil
}

func (m *mockAuditRepo) FindWithParams(ctx context.Context, params repository.AuditQueryParams) (*repository.AuditQueryResult, error) {
	return &repository.AuditQueryResult{
		Logs:       m.logs,
		NextCursor: "",
		HasMore:    false,
		TriggerIdx: 0,
	}, nil
}

func (m *mockAuditRepo) FindByID(ctx context.Context, idStr string) (*models.AuditLog, error) {
	for _, l := range m.logs {
		if l.ID.Hex() == idStr {
			return &l, nil
		}
	}
	return nil, nil
}

func (m *mockAuditRepo) GetStats(ctx context.Context) (*repository.AuditStats, error) {
	return &m.stats, nil
}

var (
	testOnce    sync.Once
	testHandler *handlers.AuditHandler
	testRepo    *mockAuditRepo
)

func setupTestAuditHandler(t *testing.T) (*handlers.AuditHandler, *mockAuditRepo) {
	t.Helper()
	testOnce.Do(func() {
		dir, _ := os.Getwd()
		for dir != "/" && dir != "." {
			if _, err := os.Stat(filepath.Join(dir, "web", "templates")); err == nil {
				_ = os.Chdir(dir)
				break
			}
			dir = filepath.Dir(dir)
		}

		renderer, err := render.NewRenderer(render.DefaultFuncMap())
		if err != nil {
			t.Fatalf("failed to create renderer: %v", err)
		}

		repo := &mockAuditRepo{
			logs: []models.AuditLog{
				{
					ID:         bson.NewObjectID(),
					Timestamp:  time.Now(),
					Actor:      "manager",
					Role:       "super_admin",
					Action:     "report.overwrite",
					ResourceID: "2026-08-01",
					IPAddress:  "127.0.0.1",
					OldState: map[string]any{
						"report_date": "2026-08-01",
						"total_sale":  100000.0,
					},
					NewState: map[string]any{
						"report_date": "2026-08-01",
						"total_sale":  120000.0,
					},
				},
			},
			stats: repository.AuditStats{
				TotalEvents:    1,
				SecurityEvents: 0,
				MutationEvents: 1,
				UniqueActors:   1,
			},
		}

		service := services.NewAuditService(repo)
		testHandler = handlers.NewAuditHandler(service, renderer)
		testRepo = repo
	})

	if testHandler == nil {
		t.Fatal("testHandler initialization failed")
	}
	return testHandler, testRepo
}


func TestAuditHandler_RenderAuditList_Unauthenticated(t *testing.T) {
	handler, _ := setupTestAuditHandler(t)

	req := httptest.NewRequest("GET", "/admin/audit", nil)
	rec := httptest.NewRecorder()

	handler.RenderAuditList(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for unauthenticated user, got %d", rec.Code)
	}
}

func TestAuditHandler_RenderAuditList_SuperAdmin(t *testing.T) {
	handler, _ := setupTestAuditHandler(t)

	req := httptest.NewRequest("GET", "/admin/audit", nil)
	adminUser := &models.User{
		Username: "manager",
		Role:     models.RoleSuperAdmin,
		FullName: "Store Manager",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, adminUser)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.RenderAuditList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for super admin, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}

func TestAuditHandler_RenderAuditDetailModal(t *testing.T) {
	handler, repo := setupTestAuditHandler(t)
	targetID := repo.logs[0].ID.Hex()

	req := httptest.NewRequest("GET", "/admin/audit/detail?id="+targetID, nil)
	adminUser := &models.User{
		Username: "manager",
		Role:     models.RoleSuperAdmin,
		FullName: "Store Manager",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, adminUser)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.RenderAuditDetailModal(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for modal inspection, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Error("expected non-empty response body for modal")
	}
}
