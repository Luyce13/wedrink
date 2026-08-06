package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"wedrink/internal/models"
	"wedrink/internal/repository"
)

type AuthService struct {
	userRepo     repository.UserRepo
	auditService *AuditService
}

func NewAuthService(userRepo repository.UserRepo, auditService *AuditService) *AuthService {
	return &AuthService{userRepo: userRepo, auditService: auditService}
}

func (s *AuthService) Authenticate(ctx context.Context, username, password string, req *http.Request) (*models.User, error) {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	user, err := s.userRepo.FindByUsername(ctx, cleanUsername)
	if err != nil {
		return nil, fmt.Errorf("authentication error: %w", err)
	}

	if user == nil {
		if s.auditService != nil {
			s.auditService.Record(ctx, RecordAuditInput{
				Actor:  cleanUsername,
				Action: "auth.login_failed",
				Req:    req,
			})
		}
		return nil, models.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		if s.auditService != nil {
			s.auditService.Record(ctx, RecordAuditInput{
				Actor:  cleanUsername,
				Action: "auth.login_failed",
				Req:    req,
			})
		}
		return nil, models.ErrInvalidCredentials
	}

	if s.auditService != nil {
		s.auditService.Record(ctx, RecordAuditInput{
			ActorID: user.ID.Hex(),
			Actor:   user.Username,
			Role:    string(user.Role),
			Action:  "auth.login_success",
			Req:     req,
		})
	}

	return user, nil
}

// VerifyPassword verifies user credentials for sensitive actions (e.g. deletions) without creating a login audit log.
func (s *AuthService) VerifyPassword(ctx context.Context, username, password string) (*models.User, error) {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	user, err := s.userRepo.FindByUsername(ctx, cleanUsername)
	if err != nil || user == nil {
		return nil, models.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, models.ErrInvalidCredentials
	}

	return user, nil
}
