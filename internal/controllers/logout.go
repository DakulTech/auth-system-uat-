package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/redis/go-redis/v9"

	"auth/internal/services"
)

type LogoutController struct {
	DB           *sql.DB
	TokenService *services.TokenService
	RedisClient  *redis.Client
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
	AllDevices   bool   `json:"all_devices"` // if true, invalidate all sessions for this user
}

// LogoutHandler invalidates refresh tokens (single device or all devices)
func (c *LogoutController) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate refresh token
	sessionToken, err := c.TokenService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Ensure device matches
	if sessionToken.DeviceID != req.DeviceID {
		http.Error(w, "Refresh token not valid for this device", http.StatusUnauthorized)
		return
	}

	// Invalidate sessions
	if req.AllDevices {
		// Delete all sessions for this user
		_, err = c.DB.Exec("DELETE FROM session_tokens WHERE user_id=$1", sessionToken.UserID)
	} else {
		// Delete only this session
		_, err = c.DB.Exec("DELETE FROM session_tokens WHERE refresh_token=$1", req.RefreshToken)
	}
	if err != nil {
		http.Error(w, "Failed to invalidate sessions", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful. Sessions invalidated."))
}
