package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	if cleanUsername == "" {
		return nil, nil
	}

	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"username": cleanUsername, "is_deleted": false}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.Username = strings.ToLower(strings.TrimSpace(user.Username))
	if user.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Pre-check to prevent duplicate usernames
	existing, err := r.FindByUsername(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("user with username '%s' already exists", user.Username)
	}

	user.CreatedAt = time.Now()
	_, err = r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("user with username '%s' already exists", user.Username)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]models.User, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"is_deleted": false})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}
	return users, nil
}

func (r *UserRepository) FindByID(ctx context.Context, idStr string) (*models.User, error) {
	objID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	var user models.User
	err = r.collection.FindOne(ctx, bson.M{"_id": objID, "is_deleted": false}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by ID: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	user.Username = strings.ToLower(strings.TrimSpace(user.Username))
	filter := bson.M{"username": user.Username, "is_deleted": false}
	if !user.ID.IsZero() {
		filter = bson.M{"_id": user.ID, "is_deleted": false}
	}

	update := bson.M{
		"$set": bson.M{
			"full_name":     user.FullName,
			"role":          user.Role,
			"password_hash": user.PasswordHash,
		},
	}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, username string, actor string) error {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"is_deleted": true,
			"deleted_at": now,
			"deleted_by": actor,
		},
	}
	res, err := r.collection.UpdateOne(ctx, bson.M{"username": cleanUsername, "is_deleted": false}, update)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) SeedDefaultUsers(ctx context.Context) error {
	// Seed super_admin user
	admin, err := r.FindByUsername(ctx, "admin")
	if err == nil && admin == nil {
		adminHash, _ := bcrypt.GenerateFromPassword([]byte("adminpassword"), bcrypt.DefaultCost)
		_ = r.Create(ctx, &models.User{
			Username:     "admin",
			PasswordHash: string(adminHash),
			FullName:     "System Administrator",
			Role:         models.RoleSuperAdmin,
		})
		slog.Info("Seeded default user: admin / adminpassword (super_admin)")
	}

	// Seed staff user
	staff, err := r.FindByUsername(ctx, "staff")
	if err == nil && staff == nil {
		staffHash, _ := bcrypt.GenerateFromPassword([]byte("staffpassword"), bcrypt.DefaultCost)
		_ = r.Create(ctx, &models.User{
			Username:     "staff",
			PasswordHash: string(staffHash),
			FullName:     "Staff Cashier",
			Role:         models.RoleStaff,
		})
		slog.Info("Seeded default user: staff / staffpassword (staff)")
	}

	return nil
}
