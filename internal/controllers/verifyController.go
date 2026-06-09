package controllers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metric for successful verifications
var (
	verifySuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "verify_success_total",
			Help: "Total number of accounts successfully verified",
		},
	)
)

func init() {
	// Register the metric once at startup
	prometheus.MustRegister(verifySuccess)
}

type VerifyController struct {
	DB *sql.DB
}

func (c *VerifyController) VerifyHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	var userID int64
	var expires time.Time
	var used bool
	err := c.DB.QueryRow(`
        SELECT user_id, expires_at, used
        FROM verification_tokens
        WHERE token = $1`, token).Scan(&userID, &expires, &used)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	if used || time.Now().After(expires) {
		http.Error(w, "Token expired or already used", http.StatusBadRequest)
		return
	}

	_, err = c.DB.Exec("UPDATE users SET verified = true WHERE id = $1", userID)
	if err != nil {
		http.Error(w, "Failed to verify user", http.StatusInternalServerError)
		return
	}

	_, _ = c.DB.Exec("UPDATE verification_tokens SET used = true WHERE token = $1", token)

	// Increment Prometheus metric
	verifySuccess.Inc()

	w.Write([]byte("Account verified successfully!"))
}
