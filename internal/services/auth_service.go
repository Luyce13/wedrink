package services

import (
	"context"
	"fmt"
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

func (s *AuthService) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
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
			})
		}
		return nil, models.ErrInvalidCredentials
	}

	if s.auditService != nil {
		s.auditService.Record(ctx, RecordAuditInput{
			Actor:  user.Username,
			Role:   string(user.Role),
			Action: "auth.login_success",
		})
	}

	return user, nil
}
