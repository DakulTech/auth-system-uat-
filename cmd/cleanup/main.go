package main

import (
	"auth/internal/config"
	"auth/internal/services"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metric
var (
	tokensCleaned = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tokens_cleaned_total",
			Help: "Total number of expired/used tokens cleaned up",
		},
	)
)

func init() {
	prometheus.MustRegister(tokensCleaned)
}

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	cfg := config.LoadWorkerConfig()

	// Connect to Postgres
	db, err := sql.Open("postgres", cfg.DBConn)
	if err != nil {
		log.Fatal("Failed to connect to Postgres:", err)
	}
	defer db.Close()

	tokenService := &services.TokenService{DB: db}

	// Start Prometheus metrics endpoint
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Println("Cleanup metrics server running on :8080/metrics")
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatal("Metrics server failed:", err)
		}
	}()

	log.Println("Cleanup worker started...")

	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Running token cleanup...")
		n, err := tokenService.CleanupExpiredTokens()
		if err != nil {
			log.Println("Cleanup failed:", err)
		} else {
			tokensCleaned.Add(float64(n))
			log.Printf("Cleanup completed successfully, %d tokens cleaned\n", n)
		}
	}
}
