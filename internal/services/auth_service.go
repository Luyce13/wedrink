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
	userRepo repository.UserRepo
}

func NewAuthService(userRepo repository.UserRepo) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	user, err := s.userRepo.FindByUsername(ctx, cleanUsername)
	if err != nil {
		return nil, fmt.Errorf("authentication error: %w", err)
	}

	if user == nil {
		return nil, models.ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, models.ErrInvalidCredentials
	}

	return user, nil
}
