package services

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"wedrink/internal/models"
	"wedrink/internal/repository"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService(userRepo *repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]models.User, error) {
	return s.userRepo.FindAll(ctx)
}

func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.userRepo.FindByUsername(ctx, username)
}

func (s *UserService) CreateUser(ctx context.Context, username, password, confirmPassword, fullName, role string) (*models.User, error) {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	if cleanUsername == "" {
		return nil, fmt.Errorf("username is required")
	}

	cleanPassword := strings.TrimSpace(password)
	cleanConfirm := strings.TrimSpace(confirmPassword)

	if cleanPassword == "" {
		return nil, fmt.Errorf("password is required")
	}
	if cleanPassword != cleanConfirm {
		return nil, fmt.Errorf("password and confirm password do not match")
	}
	if len(cleanPassword) < 4 {
		return nil, fmt.Errorf("password must be at least 4 characters")
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

	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, currentAdminUsername, targetUsername, fullName, role, newPassword, confirmPassword, adminPassword string) (*models.User, error) {
	cleanTarget := strings.ToLower(strings.TrimSpace(targetUsername))
	user, err := s.userRepo.FindByUsername(ctx, cleanTarget)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
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
			return nil, fmt.Errorf("new password and confirm password do not match")
		}
		if len(cleanNewPass) < 4 {
			return nil, fmt.Errorf("new password must be at least 4 characters")
		}

		// Verify current admin's password before allowing password reset/change
		cleanAdmin := strings.ToLower(strings.TrimSpace(currentAdminUsername))
		adminUser, errAdmin := s.userRepo.FindByUsername(ctx, cleanAdmin)
		if errAdmin != nil || adminUser == nil {
			return nil, fmt.Errorf("failed to verify admin credentials")
		}

		cleanAdminPass := strings.TrimSpace(adminPassword)
		if cleanAdminPass == "" {
			return nil, fmt.Errorf("current admin password is required to change or reset password")
		}

		errPass := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(cleanAdminPass))
		if errPass != nil {
			return nil, fmt.Errorf("incorrect current admin password. Password update denied")
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

	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, targetUsername, currentAdminUsername, adminPassword string) error {
	cleanTarget := strings.ToLower(strings.TrimSpace(targetUsername))
	cleanAdmin := strings.ToLower(strings.TrimSpace(currentAdminUsername))

	if cleanTarget == cleanAdmin {
		return fmt.Errorf("you cannot delete your own admin account")
	}

	cleanAdminPass := strings.TrimSpace(adminPassword)
	if cleanAdminPass == "" {
		return fmt.Errorf("current admin password is required to delete a user")
	}

	adminUser, errAdmin := s.userRepo.FindByUsername(ctx, cleanAdmin)
	if errAdmin != nil || adminUser == nil {
		return fmt.Errorf("failed to verify admin credentials")
	}

	errPass := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(cleanAdminPass))
	if errPass != nil {
		return fmt.Errorf("incorrect admin password. User deletion denied")
	}

	return s.userRepo.Delete(ctx, cleanTarget)
}
