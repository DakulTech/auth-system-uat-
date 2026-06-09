package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"

	"github.com/redis/go-redis/v9"

	"auth/internal/services"
)

// Prometheus counters
var (
	resetPasswordRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "reset_password_requests_total",
			Help: "Total number of reset password requests",
		},
	)
	resetPasswordSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "reset_password_success_total",
			Help: "Total number of successful password resets",
		},
	)
	resetPasswordFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "reset_password_failed_total",
			Help: "Total number of failed password resets",
		},
	)
)

func init() {
	prometheus.MustRegister(resetPasswordRequests, resetPasswordSuccess, resetPasswordFailed)
}

type ResetPasswordController struct {
	DB           *sql.DB
	TokenService *services.TokenService
	RedisClient  *redis.Client
}

// Request payloads
type ResetRequest struct {
	Email string `json:"email"`
}

type ResetConfirm struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// EmailJob matches the worker’s expected format
type EmailJob struct {
	Type      string `json:"type"`
	Email     string `json:"email"`
	UserID    int64  `json:"user_id"`
	Token     string `json:"token"`
	Timestamp int64  `json:"timestamp"`
}

// Step 1: Request reset link
func (c *ResetPasswordController) RequestResetHandler(w http.ResponseWriter, r *http.Request) {
	resetPasswordRequests.Inc()

	var req ResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		resetPasswordFailed.Inc()
		return
	}

	// Generate token and store in DB
	resetToken, err := c.TokenService.GenerateResetToken(req.Email, 15*time.Minute)
	if err != nil {
		http.Error(w, "Unable to generate reset token", http.StatusInternalServerError)
		resetPasswordFailed.Inc()
		return
	}

	// Enqueue job for worker
	job := EmailJob{
		Type:      "reset_password",
		Email:     req.Email,
		UserID:    0,                // reset is email-based
		Token:     resetToken.Token, // use the string field
		Timestamp: time.Now().Unix(),
	}
	payload, _ := json.Marshal(job)
	if err := c.RedisClient.LPush(context.Background(), "jobs:email", payload).Err(); err != nil {
		http.Error(w, "Failed to enqueue reset job", http.StatusInternalServerError)
		resetPasswordFailed.Inc()
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Reset link queued for delivery"))
}

// Step 2: Confirm reset with new password
func (c *ResetPasswordController) ConfirmResetHandler(w http.ResponseWriter, r *http.Request) {
	var req ResetConfirm
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		resetPasswordFailed.Inc()
		return
	}

	// Validate token
	resetToken, err := c.TokenService.ValidateResetToken(req.Token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		resetPasswordFailed.Inc()
		return
	}

	// Hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		resetPasswordFailed.Inc()
		return
	}

	// Update user password securely
	_, err = c.DB.Exec("UPDATE users SET password_hash=$1 WHERE email=$2", string(hashed), resetToken.Email)
	if err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		resetPasswordFailed.Inc()
		return
	}

	// Invalidate all sessions for this user (global logout)
	_, err = c.DB.Exec("DELETE FROM session_tokens WHERE user_id = (SELECT id FROM users WHERE email=$1)", resetToken.Email)
	if err != nil {
		http.Error(w, "Failed to invalidate sessions", http.StatusInternalServerError)
		resetPasswordFailed.Inc()
		return
	}

	resetPasswordSuccess.Inc()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Password reset successful. All sessions invalidated."))
}
