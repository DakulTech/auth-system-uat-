package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"auth/internal/config"
)

type EmailJob struct {
	Type      string `json:"type"`
	Email     string `json:"email"`
	UserID    int64  `json:"user_id"`
	Token     string `json:"token"`
	Timestamp int64  `json:"timestamp"`
}

// Prometheus metrics
var (
	jobsProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "jobs_processed_total",
			Help: "Total number of jobs processed successfully",
		},
	)
	jobsFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "jobs_failed_total",
			Help: "Total number of jobs that failed",
		},
	)
)

func init() {
	prometheus.MustRegister(jobsProcessed, jobsFailed)
}

func sendVerificationEmail(cfg *config.WorkerConfig, to string, userID int64) error {
	subject := "Subject: Verify your account\n"
	body := fmt.Sprintf("Hello,\n\nPlease verify your account (user_id=%d).\n\nThanks!", userID)
	msg := []byte(subject + "\n" + body)

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	return smtp.SendMail(cfg.SMTPHost+":"+cfg.SMTPPort, auth, cfg.SMTPUser, []string{to}, msg)
}

func sendResetEmail(cfg *config.WorkerConfig, to string, token string) error {
	subject := "Subject: Reset your password\n"
	resetLink := fmt.Sprintf("https://yourapp.com/reset-password?token=%s", token)
	body := fmt.Sprintf("Hello,\n\nClick the following link to reset your password:\n%s\n\nThanks!", resetLink)
	msg := []byte(subject + "\n" + body)

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	return smtp.SendMail(cfg.SMTPHost+":"+cfg.SMTPPort, auth, cfg.SMTPUser, []string{to}, msg)
}

func markUserVerified(db *sql.DB, userID int64) {
	_, err := db.Exec("UPDATE users SET verified = true WHERE id = $1", userID)
	if err != nil {
		log.Printf("Failed to mark user %d as verified: %v\n", userID, err)
	} else {
		log.Printf("User %d marked as verified\n", userID)
	}
}

func main() {
	cfg := config.LoadWorkerConfig()
	ctx := context.Background()

	// Start Prometheus metrics endpoint
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Println("Metrics server running on :8080/metrics")
		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatal("Metrics server failed:", err)
		}
	}()

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	// Connect to Postgres
	db, err := sql.Open("postgres", cfg.DBConn)
	if err != nil {
		log.Fatal("Failed to connect to Postgres:", err)
	}
	defer db.Close()

	// Check Redis availability
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Println("Warning: Redis not reachable at startup, worker will rely on DB polling")
	}

	log.Println("Worker started, listening for jobs...")

	for {
		// Try Redis first
		result, err := rdb.BRPop(ctx, 5*time.Second, "jobs:email").Result()
		if err != nil {
			log.Println("Redis unavailable, falling back to DB polling:", err)

			rows, err := db.Query("SELECT id, email FROM users WHERE verified = false LIMIT 5")
			if err != nil {
				log.Println("DB fallback failed:", err)
				time.Sleep(5 * time.Second)
				continue
			}
			for rows.Next() {
				var id int64
				var email string
				if err := rows.Scan(&id, &email); err != nil {
					log.Println("Row scan error:", err)
					continue
				}
				log.Printf("Fallback: sending verification email to %s (user_id=%d)\n", email, id)
				if err := sendVerificationEmail(cfg, email, id); err != nil {
					log.Println("Failed to send email:", err)
					jobsFailed.Inc()
				} else {
					markUserVerified(db, id)
					jobsProcessed.Inc()
				}
			}
			rows.Close()
			continue
		}

		if len(result) < 2 {
			continue
		}

		var job EmailJob
		if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
			log.Println("Failed to parse job:", err)
			jobsFailed.Inc()
			continue
		}

		switch job.Type {
		case "email_verification":
			log.Printf("Sending verification email to %s (user_id=%d)\n", job.Email, job.UserID)
			if err := sendVerificationEmail(cfg, job.Email, job.UserID); err != nil {
				log.Println("Failed to send verification email:", err)
				jobsFailed.Inc()
			} else {
				markUserVerified(db, job.UserID)
				jobsProcessed.Inc()
			}

		case "reset_password":
			log.Printf("Sending reset email to %s (token=%s)\n", job.Email, job.Token)
			if err := sendResetEmail(cfg, job.Email, job.Token); err != nil {
				log.Println("Failed to send reset email:", err)
				jobsFailed.Inc()
			} else {
				log.Printf("Reset email sent to %s\n", job.Email)
				jobsProcessed.Inc()
			}

		default:
			log.Println("Unknown job type:", job.Type)
		}
	}
}
