package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"

	"wedrink/internal/models"
)

type UserRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		collection: db.Collection("users"),
	}
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepository) SeedDefaultUsers(ctx context.Context) error {
	// Seed super_admin user
	admin, err := r.FindByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	if admin == nil {
		adminHash, _ := bcrypt.GenerateFromPassword([]byte("adminpassword"), bcrypt.DefaultCost)
		err = r.Create(ctx, &models.User{
			Username:     "admin",
			PasswordHash: string(adminHash),
			FullName:     "System Administrator",
			Role:         models.RoleSuperAdmin,
		})
		if err != nil {
			slog.Warn("Could not seed super_admin user", "error", err)
		} else {
			slog.Info("Seeded default user: admin / adminpassword (super_admin)")
		}
	}

	// Seed staff user
	staff, err := r.FindByUsername(ctx, "staff")
	if err != nil {
		return err
	}
	if staff == nil {
		staffHash, _ := bcrypt.GenerateFromPassword([]byte("staffpassword"), bcrypt.DefaultCost)
		err = r.Create(ctx, &models.User{
			Username:     "staff",
			PasswordHash: string(staffHash),
			FullName:     "Store Staff Member",
			Role:         models.RoleStaff,
		})
		if err != nil {
			slog.Warn("Could not seed staff user", "error", err)
		} else {
			slog.Info("Seeded default user: staff / staffpassword (staff)")
		}
	}

	return nil
}
