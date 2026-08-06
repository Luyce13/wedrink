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

type UserService struct {
	userRepo     repository.UserRepo
	auditService *AuditService
}

func NewUserService(userRepo repository.UserRepo, auditService *AuditService) *UserService {
	return &UserService{userRepo: userRepo, auditService: auditService}
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]models.User, error) {
	return s.userRepo.FindAll(ctx)
}

func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.userRepo.FindByUsername(ctx, username)
}

func (s *UserService) CreateUser(ctx context.Context, username, password, confirmPassword, fullName, role string, req *http.Request) (*models.User, error) {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	if cleanUsername == "" {
		return nil, models.ErrUsernameRequired
	}

	cleanPassword := strings.TrimSpace(password)
	cleanConfirm := strings.TrimSpace(confirmPassword)

	if cleanPassword == "" {
		return nil, models.ErrPasswordRequired
	}
	if cleanPassword != cleanConfirm {
		return nil, models.ErrPasswordMismatch
	}
	if len(cleanPassword) < 4 {
		return nil, models.ErrPasswordTooShort
	}

	cleanFullName := strings.TrimSpace(fullName)
	if cleanFullName == "" {
		cleanFullName = cleanUsername
	}

	userRole := models.RoleStaff
	if role == string(models.RoleSuperAdmin) || role == "manager" || role == "admin" {
		userRole = models.RoleSuperAdmin
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cleanPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("password hashing failed: %w", err)
	}

	user := &models.User{
		Username:     cleanUsername,
		PasswordHash: string(hash),
		FullName:     cleanFullName,
		Role:         userRole,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	if s.auditService != nil {
		s.auditService.Record(ctx, RecordAuditInput{
			Action:     "user.create",
			ResourceID: user.Username,
			Req:        req,
			NewState:   user,
		})
	}

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, currentAdminUsername, targetUsername, fullName, role, newPassword, confirmPassword, adminPassword string, req *http.Request) (*models.User, error) {
	cleanTarget := strings.ToLower(strings.TrimSpace(targetUsername))
	user, err := s.userRepo.FindByUsername(ctx, cleanTarget)
	if err != nil || user == nil {
		return nil, models.ErrUserNotFound
	}

	if strings.TrimSpace(fullName) != "" {
		user.FullName = strings.TrimSpace(fullName)
	}

	if role != "" {
		if role == string(models.RoleSuperAdmin) || role == "manager" || role == "admin" {
			user.Role = models.RoleSuperAdmin
		} else {
			user.Role = models.RoleStaff
		}
	}

	cleanNewPass := strings.TrimSpace(newPassword)
	cleanConfirmPass := strings.TrimSpace(confirmPassword)

	if cleanNewPass != "" || cleanConfirmPass != "" {
		if cleanNewPass != cleanConfirmPass {
			return nil, models.ErrPasswordMismatch
		}
		if len(cleanNewPass) < 4 {
			return nil, models.ErrPasswordTooShort
		}

		// Verify current admin's password before allowing password reset/change
		cleanAdmin := strings.ToLower(strings.TrimSpace(currentAdminUsername))
		adminUser, errAdmin := s.userRepo.FindByUsername(ctx, cleanAdmin)
		if errAdmin != nil || adminUser == nil {
			return nil, fmt.Errorf("failed to verify admin credentials")
		}

		cleanAdminPass := strings.TrimSpace(adminPassword)
		if cleanAdminPass == "" {
			return nil, models.ErrAdminPasswordRequired
		}

		errPass := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(cleanAdminPass))
		if errPass != nil {
			return nil, models.ErrIncorrectAdminPassword
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(cleanNewPass), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("password hashing failed: %w", err)
		}
		user.PasswordHash = string(hash)
	}

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return nil, err
	}

	if s.auditService != nil {
		s.auditService.Record(ctx, RecordAuditInput{
			Actor:      currentAdminUsername,
			Action:     "user.update",
			ResourceID: user.Username,
			Req:        req,
			NewState:   user,
		})
	}

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, targetUsername, currentAdminUsername, adminPassword string, req *http.Request) error {
	cleanTarget := strings.ToLower(strings.TrimSpace(targetUsername))
	cleanAdmin := strings.ToLower(strings.TrimSpace(currentAdminUsername))

	if cleanTarget == cleanAdmin {
		return models.ErrCannotDeleteSelf
	}

	cleanAdminPass := strings.TrimSpace(adminPassword)
	if cleanAdminPass == "" {
		return models.ErrAdminPasswordRequired
	}

	adminUser, errAdmin := s.userRepo.FindByUsername(ctx, cleanAdmin)
	if errAdmin != nil || adminUser == nil {
		return fmt.Errorf("failed to verify admin credentials")
	}

	errPass := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(cleanAdminPass))
	if errPass != nil {
		return models.ErrIncorrectAdminPassword
	}

	oldUser, _ := s.userRepo.FindByUsername(ctx, cleanTarget)
	err := s.userRepo.Delete(ctx, cleanTarget, cleanAdmin)
	if err == nil && s.auditService != nil {
		s.auditService.Record(ctx, RecordAuditInput{
			Actor:      cleanAdmin,
			Action:     "user.delete",
			ResourceID: cleanTarget,
			Req:        req,
			OldState:   oldUser,
			NewState:   nil,
		})
	}
	return err
}
