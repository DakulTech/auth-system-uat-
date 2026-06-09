package models

import "time"

type User struct {
	ID          int64
	Email       string
	Password    string
	Verified    bool
	LockedUntil *time.Time
	MFAEnabled  bool
	MFASecret   string
}

type VerificationToken struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

type PasswordResetToken struct {
	ID        int64
	Email     string
	Token     string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

type SessionToken struct {
	ID           int64
	UserID       int64
	DeviceID     string
	RefreshToken string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}
