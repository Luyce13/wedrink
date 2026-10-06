package services_test

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/models"
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

func TestAuditServiceQueryAndStats(t *testing.T) {
	id := bson.NewObjectID()
	mockRepo := &mockAuditRepo{
		logs: []models.AuditLog{
			{
				ID:        id,
				Timestamp: time.Now(),
				Actor:     "manager",
				Role:      "super_admin",
				Action:    "report.overwrite",
			},
		},
		stats: repository.AuditStats{
			TotalEvents:    1,
			SecurityEvents: 0,
			MutationEvents: 1,
			UniqueActors:   1,
		},
	}

	svc := services.NewAuditService(mockRepo)
	ctx := context.Background()

	// 1. Test GetAuditLogs
	res, err := svc.GetAuditLogs(ctx, repository.AuditQueryParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(res.Logs))
	}
	if res.Logs[0].Action != "report.overwrite" {
		t.Errorf("expected report.overwrite, got %s", res.Logs[0].Action)
	}

	// 2. Test GetAuditLogByID
	found, err := svc.GetAuditLogByID(ctx, id.Hex())
	if err != nil {
		t.Fatalf("unexpected error finding by id: %v", err)
	}
	if found == nil || found.ID != id {
		t.Fatalf("expected to find audit log with id %s", id.Hex())
	}

	// 3. Test GetStats
	stats, err := svc.GetStats(ctx)
	if err != nil {
		t.Fatalf("unexpected error getting stats: %v", err)
	}
	if stats.TotalEvents != 1 || stats.MutationEvents != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}
