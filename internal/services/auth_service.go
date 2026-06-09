package services

import (
	"auth/internal/models"
	"auth/internal/repository"
	"errors"

	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthService struct {
	UserRepo *repository.PostgresUserRepository
	Redis    *redis.Client
	Tokens   *TokenService
}

func (s *AuthService) Register(req RegisterRequest) error {
	// Check if user exists
	existing, _ := s.UserRepo.FindByEmail(req.Email)
	if existing != nil {
		return errors.New("user already exists")
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &models.User{
		Email:    req.Email,
		Password: string(hashed),
		Verified: false,
	}

	// Save user
	if err := s.UserRepo.Create(user); err != nil {
		return err
	}

	// ✅ Generate verification token
	token, err := s.Tokens.GenerateToken(user.ID)
	if err != nil {
		return errors.New("failed to generate verification token: " + err.Error())
	}

	// Enqueue email verification job in Redis
	job := map[string]interface{}{
		"type":      "email_verification",
		"email":     user.Email,
		"user_id":   user.ID,
		"token":     token, // ✅ include token
		"timestamp": time.Now().Unix(),
	}

	payload, _ := json.Marshal(job)
	ctx := context.Background()
	if err := s.Redis.LPush(ctx, "jobs:email", payload).Err(); err != nil {
		return errors.New("failed to enqueue email verification job: " + err.Error())
	}

	return nil
}
