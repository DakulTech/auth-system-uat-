package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"auth/internal/models"
	"auth/internal/services"
)

type LoginController struct {
	DB           *sql.DB
	TokenService *services.TokenService
	RedisClient  *redis.Client
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	OTPCode  string `json:"otp_code"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	StepUpAuth   bool   `json:"step_up_auth"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}

type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}

// LoginHandler: step 1 – validate credentials, log activity, enforce MFA if provided
func (c *LoginController) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var user models.User
	err := c.DB.QueryRow(`
        SELECT id, email, password_hash, verified, locked_until, mfa_enabled
        FROM users WHERE email=$1
    `, req.Email).Scan(&user.ID, &user.Email, &user.Password, &user.Verified, &user.LockedUntil, &user.MFAEnabled)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		http.Error(w, "Account temporarily locked", http.StatusTooManyRequests)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		key := fmt.Sprintf("login:fail:%d", user.ID)
		fails, _ := c.RedisClient.Incr(ctx, key).Result()
		if fails == 1 {
			c.RedisClient.Expire(ctx, key, 10*time.Minute)
		}
		if fails > 5 {
			c.DB.Exec("UPDATE users SET locked_until = NOW() + interval '15 minutes' WHERE id=$1", user.ID)
		}
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	c.RedisClient.Del(ctx, fmt.Sprintf("login:fail:%d", user.ID))

	ip := req.IP
	if ip == "" {
		ip = r.RemoteAddr
	}
	device := req.DeviceID
	_, _ = c.DB.Exec("INSERT INTO login_activity (user_id, ip_address, device) VALUES ($1, $2, $3)",
		user.ID, ip, device)

	// If MFA is enabled, require OTP via /login/otp
	if user.MFAEnabled {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Credentials valid. Please complete OTP verification."))
		return
	}

	// If no MFA, issue tokens directly
	accessToken, err := c.TokenService.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := c.TokenService.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	_, _ = c.DB.Exec(`
        INSERT INTO session_tokens (user_id, device_id, refresh_token, expires_at)
        VALUES ($1, $2, $3, NOW() + interval '30 days')
    `, user.ID, device, refreshToken)

	resp := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		StepUpAuth:   false,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// OTPHandler: step 2 – validate OTP and issue tokens
func (c *LoginController) OTPHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		OTPCode  string `json:"otp_code"`
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var user models.User
	err := c.DB.QueryRow(`SELECT id, email, mfa_secret, mfa_enabled FROM users WHERE email=$1`,
		req.Email).Scan(&user.ID, &user.Email, &user.MFASecret, &user.MFAEnabled)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	if !user.MFAEnabled {
		http.Error(w, "MFA not enabled for this user", http.StatusBadRequest)
		return
	}

	if !c.TokenService.ValidateOTP(user.ID, req.OTPCode) {
		http.Error(w, "Invalid OTP", http.StatusUnauthorized)
		return
	}

	accessToken, err := c.TokenService.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}
	refreshToken, err := c.TokenService.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	_, _ = c.DB.Exec(`
        INSERT INTO session_tokens (user_id, device_id, refresh_token, expires_at)
        VALUES ($1, $2, $3, NOW() + interval '30 days')
    `, user.ID, req.DeviceID, refreshToken)

	resp := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		StepUpAuth:   false,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RefreshHandler: renew access token
func (c *LoginController) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	sessionToken, err := c.TokenService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	if sessionToken.DeviceID != req.DeviceID {
		http.Error(w, "Refresh token not valid for this device", http.StatusUnauthorized)
		return
	}

	accessToken, err := c.TokenService.GenerateAccessToken(sessionToken.UserID, "")
	if err != nil {
		http.Error(w, "Failed to generate new access token", http.StatusInternalServerError)
		return
	}

	resp := RefreshResponse{AccessToken: accessToken}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
