package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"wedrink/internal/models"
	"wedrink/internal/repository"
	"wedrink/internal/utils"
)

type AuditService struct {
	repo repository.AuditRepo
}

func NewAuditService(repo repository.AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

type RecordAuditInput struct {
	ActorID    string
	Actor      string
	Role       string
	Action     string
	ResourceID string
	Req        *http.Request
	OldState   any
	NewState   any
}

func (s *AuditService) Record(ctx context.Context, input RecordAuditInput) {
	if s == nil || s.repo == nil {
		return
	}

	ipAddr := "127.0.0.1"
	userAgent := ""
	if input.Req != nil {
		ipAddr = utils.GetClientIP(input.Req)
		userAgent = input.Req.UserAgent()
	}

	actorID := input.ActorID
	if actorID == "000000000000000000000000" {
		actorID = ""
	}

	log := &models.AuditLog{
		Timestamp:  time.Now(),
		ActorID:    actorID,
		Actor:      input.Actor,
		Role:       input.Role,
		Action:     input.Action,
		ResourceID: input.ResourceID,
		IPAddress:  ipAddr,
		UserAgent:  userAgent,
		OldState:   input.OldState,
		NewState:   input.NewState,
	}

	go func() {
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.Create(logCtx, log); err != nil {
			slog.Error("Failed to persist audit log", "action", input.Action, "actor", input.Actor, "error", err)
		}
	}()
}
