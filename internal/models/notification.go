package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Notification struct {
	ID          bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	ReportID    string        `json:"report_id" bson:"report_id"`
	ReportDate  string        `json:"report_date" bson:"report_date"`
	SubmittedBy string        `json:"submitted_by" bson:"submitted_by"`
	Notes       string        `json:"notes" bson:"notes"`
	IsRead      bool          `json:"is_read" bson:"is_read"`
	CreatedAt   time.Time     `json:"created_at" bson:"created_at"`
}
