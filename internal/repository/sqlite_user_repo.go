package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"wedrink/internal/models"
)

type SQLiteUserRepository struct {
	db *sql.DB
}

func NewSQLiteUserRepository(db *sql.DB) *SQLiteUserRepository {
	return &SQLiteUserRepository{db: db}
}

func (r *SQLiteUserRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	if cleanUsername == "" {
		return nil, nil
	}

	query := `
		SELECT id, username, password_hash, full_name, role, is_deleted, deleted_at, deleted_by, created_at
		FROM users
		WHERE username = ? COLLATE NOCASE AND is_deleted = 0
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, cleanUsername)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by username: %w", err)
	}
	return user, nil
}

func (r *SQLiteUserRepository) FindByID(ctx context.Context, idStr string) (*models.User, error) {
	cleanID := strings.TrimSpace(idStr)
	if cleanID == "" {
		return nil, nil
	}

	query := `
		SELECT id, username, password_hash, full_name, role, is_deleted, deleted_at, deleted_by, created_at
		FROM users
		WHERE id = ? AND is_deleted = 0
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, cleanID)

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}
	return user, nil
}

func (r *SQLiteUserRepository) Create(ctx context.Context, user *models.User) error {
	user.Username = strings.ToLower(strings.TrimSpace(user.Username))
	if user.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Pre-check for duplicate username
	existing, err := r.FindByUsername(ctx, user.Username)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("user with username '%s' already exists", user.Username)
	}

	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO users (id, username, password_hash, full_name, role, is_deleted, deleted_at, deleted_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	var deletedAtStr *string
	if user.DeletedAt != nil {
		s := user.DeletedAt.Format(time.RFC3339)
		deletedAtStr = &s
	}

	isDel := 0
	if user.IsDeleted {
		isDel = 1
	}

	_, err = r.db.ExecContext(ctx, query,
		user.ID.Hex(),
		user.Username,
		user.PasswordHash,
		user.FullName,
		string(user.Role),
		isDel,
		deletedAtStr,
		user.DeletedBy,
		user.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("user with username '%s' already exists", user.Username)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *SQLiteUserRepository) FindAll(ctx context.Context) ([]models.User, error) {
	query := `
		SELECT id, username, password_hash, full_name, role, is_deleted, deleted_at, deleted_by, created_at
		FROM users
		WHERE is_deleted = 0
		ORDER BY created_at ASC;
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("users iteration error: %w", err)
	}
	return users, nil
}

func (r *SQLiteUserRepository) Update(ctx context.Context, user *models.User) error {
	user.Username = strings.ToLower(strings.TrimSpace(user.Username))
	query := `
		UPDATE users
		SET full_name = ?, role = ?, password_hash = ?
		WHERE (id = ? OR username = ? COLLATE NOCASE) AND is_deleted = 0;
	`
	idHex := ""
	if !user.ID.IsZero() {
		idHex = user.ID.Hex()
	}

	res, err := r.db.ExecContext(ctx, query,
		user.FullName,
		string(user.Role),
		user.PasswordHash,
		idHex,
		user.Username,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *SQLiteUserRepository) Delete(ctx context.Context, username string, actor string) error {
	cleanUsername := strings.ToLower(strings.TrimSpace(username))
	now := time.Now().Format(time.RFC3339)

	query := `
		UPDATE users
		SET is_deleted = 1, deleted_at = ?, deleted_by = ?
		WHERE username = ? COLLATE NOCASE AND is_deleted = 0;
	`
	res, err := r.db.ExecContext(ctx, query, now, actor, cleanUsername)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *SQLiteUserRepository) SeedDefaultUsers(ctx context.Context) error {
	admin, err := r.FindByUsername(ctx, "admin")
	if err == nil && admin == nil {
		adminHash, _ := bcrypt.GenerateFromPassword([]byte("adminpassword"), bcrypt.DefaultCost)
		_ = r.Create(ctx, &models.User{
			Username:     "admin",
			PasswordHash: string(adminHash),
			FullName:     "System Administrator",
			Role:         models.RoleSuperAdmin,
		})
		slog.Info("Seeded default user in SQLite: admin / adminpassword (super_admin)")
	}

	staff, err := r.FindByUsername(ctx, "staff")
	if err == nil && staff == nil {
		staffHash, _ := bcrypt.GenerateFromPassword([]byte("staffpassword"), bcrypt.DefaultCost)
		_ = r.Create(ctx, &models.User{
			Username:     "staff",
			PasswordHash: string(staffHash),
			FullName:     "Staff Cashier",
			Role:         models.RoleStaff,
		})
		slog.Info("Seeded default user in SQLite: staff / staffpassword (staff)")
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner rowScanner) (*models.User, error) {
	var (
		idStr        string
		username     string
		passwordHash string
		fullName     string
		roleStr      string
		isDeletedInt int
		deletedAtStr sql.NullString
		deletedBy    sql.NullString
		createdAtStr string
	)

	err := scanner.Scan(
		&idStr,
		&username,
		&passwordHash,
		&fullName,
		&roleStr,
		&isDeletedInt,
		&deletedAtStr,
		&deletedBy,
		&createdAtStr,
	)
	if err != nil {
		return nil, err
	}

	objID, _ := bson.ObjectIDFromHex(idStr)
	createdAt, _ := time.Parse(time.RFC3339, createdAtStr)

	var deletedAt *time.Time
	if deletedAtStr.Valid {
		if t, err := time.Parse(time.RFC3339, deletedAtStr.String); err == nil {
			deletedAt = &t
		}
	}

	return &models.User{
		ID:           objID,
		Username:     username,
		PasswordHash: passwordHash,
		FullName:     fullName,
		Role:         models.Role(roleStr),
		IsDeleted:    isDeletedInt == 1,
		DeletedAt:    deletedAt,
		DeletedBy:    deletedBy.String,
		CreatedAt:    createdAt,
	}, nil
}
