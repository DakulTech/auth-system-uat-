package controllers

import (
	"encoding/json"
	"net/http"

	"auth/internal/services"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metric for registration requests
var (
	registerRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "register_requests_total",
			Help: "Total number of registration requests received",
		},
	)
)

func init() {
	// Register the metric once at startup
	prometheus.MustRegister(registerRequests)
}

type RegisterController struct {
	AuthService *services.AuthService
}

func (c *RegisterController) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// Increment metric for every request
	registerRequests.Inc()

	var req services.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := c.AuthService.Register(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User registered successfully, check email for verification"))
}
