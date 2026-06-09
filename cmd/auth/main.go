package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"auth/internal/config"
	"auth/internal/middleware"
	"auth/internal/repository"
	"auth/internal/routes"
	"auth/internal/services"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	cfg := config.LoadAPIConfig()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", cfg.DBConn)
	if err != nil {
		log.Fatal("Failed to connect to Postgres:", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("Unable to reach Postgres:", err)
	}
	defer db.Close()

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Println("Warning: Redis not reachable, continuing without idempotency middleware")
	}

	// Initialize services
	userRepo := repository.NewPostgresUserRepository(db)
	tokenService := &services.TokenService{DB: db}
	authService := &services.AuthService{
		UserRepo: userRepo,
		Redis:    rdb,
		Tokens:   tokenService,
	}

	// Router
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Register endpoint with idempotency middleware
	var registerHandler http.Handler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Delegate to controller
			routes.RegisterRoutes(mux, authService)
		},
	)
	if err := rdb.Ping(context.Background()).Err(); err == nil {
		idem := middleware.NewIdempotencyMiddleware(rdb, 5*time.Minute)
		registerHandler = idem.Handler(registerHandler)
	}
	mux.Handle("/register", registerHandler)

	// Verify endpoint
	routes.VerifyRoutes(mux, db)

	// Reset password endpoints
	routes.RegisterResetPasswordRoutes(mux, db, tokenService, rdb)

	// Logout endpoint
	routes.RegisterLogoutRoutes(mux, db, tokenService, rdb)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Start server
	log.Printf("Auth API running on :%s\n", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, mux); err != nil {
		log.Fatal("Server failed:", err)
	}
}
