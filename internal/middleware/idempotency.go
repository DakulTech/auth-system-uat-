package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type IdempotencyMiddleware struct {
	Redis *redis.Client
	TTL   time.Duration
}

func NewIdempotencyMiddleware(rdb *redis.Client, ttl time.Duration) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{Redis: rdb, TTL: ttl}
}

func (m *IdempotencyMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			http.Error(w, "Missing Idempotency-Key header", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		redisKey := "idem:" + key

		// Try to set the key if it doesn't exist
		set, err := m.Redis.SetNX(ctx, redisKey, "used", m.TTL).Result()
		if err != nil {
			http.Error(w, "Redis error", http.StatusInternalServerError)
			return
		}

		if !set {
			// Key already exists → duplicate request
			http.Error(w, "Duplicate request detected", http.StatusConflict)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}
