package services

// EmailJob represents the payload pushed to Redis and consumed by the worker.
// It carries all the information needed to send a verification email.
type EmailJob struct {
	Type      string `json:"type"`      // e.g. "email_verification"
	Email     string `json:"email"`     // recipient email
	UserID    int64  `json:"user_id"`   // ID of the user
	Token     string `json:"token"`     // verification token
	Timestamp int64  `json:"timestamp"` // Unix timestamp when job was created
}
