package models

import "errors"

// Sentinel errors for domain rule violations.
// Use errors.Is() to check for these in handlers when you need to
// distinguish business logic failures from infrastructure errors.
var (
	// Report validation errors
	ErrReportDateRequired = errors.New("Report date is required")
	ErrDateBeforeMinimum  = errors.New("Date cannot be prior to minimum date")
	ErrFutureDate         = errors.New("Cannot submit report for a future date")
	ErrDuplicateReport    = errors.New("A report for this date already exists")
	ErrReportNotFound     = errors.New("Report not found")
	ErrInvalidAmount      = errors.New("Invalid monetary amount")
	ErrNegativeAmount     = errors.New("Monetary amounts cannot be negative")

	// Authentication errors
	ErrInvalidCredentials = errors.New("Invalid username or password")

	// User management errors
	ErrUserNotFound           = errors.New("User not found")
	ErrUsernameRequired       = errors.New("Username is required")
	ErrPasswordRequired       = errors.New("Password is required")
	ErrPasswordTooShort       = errors.New("Password must be at least 4 characters")
	ErrPasswordMismatch       = errors.New("Password and confirm password do not match")
	ErrDuplicateUsername      = errors.New("Username already exists")
	ErrCannotDeleteSelf       = errors.New("You cannot delete your own account")
	ErrAdminPasswordRequired  = errors.New("Admin password is required")
	ErrIncorrectAdminPassword = errors.New("Incorrect admin password")

	// Notification errors
	ErrNotificationIDRequired = errors.New("Notification ID is required")
	ErrNotificationNotFound   = errors.New("Notification not found")
)
