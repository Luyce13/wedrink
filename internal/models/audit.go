package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuditLog represents an immutable record of a system event or data mutation.
type AuditLog struct {
	ID         bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Timestamp  time.Time     `json:"timestamp" bson:"timestamp"`
	ActorID    string        `json:"actor_id,omitempty" bson:"actor_id,omitempty"` // User ID (_id.Hex())
	Actor      string        `json:"actor" bson:"actor"`               // Username of person performing action
	Role       string        `json:"role" bson:"role"`                 // Role of actor (staff/super_admin)
	Action     string        `json:"action" bson:"action"`             // e.g. "report.submit", "report.delete", "user.create"
	ResourceID string        `json:"resource_id,omitempty" bson:"resource_id,omitempty"` // ID or key of target resource
	IPAddress  string        `json:"ip_address" bson:"ip_address"`     // Client IP (CF-Connecting-IP / RemoteAddr)
	UserAgent  string        `json:"user_agent" bson:"user_agent"`     // Client browser/device User-Agent
	OldState   any           `json:"old_state,omitempty" bson:"old_state,omitempty"` // Snapshot before mutation
	NewState   any           `json:"new_state,omitempty" bson:"new_state,omitempty"` // Snapshot after mutation
}
