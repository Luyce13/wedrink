package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleStaff      Role = "staff"
)

type User struct {
	ID           bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Username     string        `json:"username" bson:"username"`
	PasswordHash string        `json:"-" bson:"password_hash"`
	FullName     string        `json:"full_name" bson:"full_name"`
	Role         Role          `json:"role" bson:"role"`
	CreatedAt    time.Time     `json:"created_at" bson:"created_at"`
}

func (u *User) IsSuperAdmin() bool {
	return u.Role == RoleSuperAdmin
}

func (u *User) CanEditReports() bool {
	return u.Role == RoleSuperAdmin
}

func (u *User) CanExportData() bool {
	return u.Role == RoleSuperAdmin
}
