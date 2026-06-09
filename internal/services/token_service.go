package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"auth/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
)

type TokenService struct {
	DB          *sql.DB
	TokenExpiry time.Duration
	JWTSecret   []byte
}

// GenerateToken creates a random verification token, invalidates old ones, stores the new one, and returns it
func (s *TokenService) GenerateToken(userID int64) (*models.VerificationToken, error) {
	_, _ = s.DB.Exec("UPDATE verification_tokens SET used = true WHERE user_id = $1 AND used = false", userID)

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(b)

	expiry := s.TokenExpiry
	if expiry == 0 {
		expiry = 24 * time.Hour
	}
	expiresAt := time.Now().Add(expiry)

	_, err := s.DB.Exec(`
        INSERT INTO verification_tokens (user_id, token, expires_at, used)
        VALUES ($1, $2, $3, false)`,
		userID, token, expiresAt)
	if err != nil {
		return nil, err
	}

	return &models.VerificationToken{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}, nil
}

func (s *TokenService) MarkTokenUsed(token string) error {
	_, err := s.DB.Exec("UPDATE verification_tokens SET used = true WHERE token = $1", token)
	return err
}

func (s *TokenService) CleanupExpiredTokens() (int64, error) {
	var total int64

	result, err := s.DB.Exec("DELETE FROM verification_tokens WHERE expires_at < NOW() OR used = true")
	if err != nil {
		return 0, err
	}
	rowsAffected, _ := result.RowsAffected()
	total += rowsAffected

	result, err = s.DB.Exec("DELETE FROM password_reset_tokens WHERE expires_at < NOW() OR used = true")
	if err != nil {
		return total, err
	}
	rowsAffected, _ = result.RowsAffected()
	total += rowsAffected

	result, err = s.DB.Exec("DELETE FROM session_tokens WHERE expires_at < NOW()")
	if err == nil {
		rowsAffected, _ = result.RowsAffected()
		total += rowsAffected
	}

	return total, nil
}

func (s *TokenService) GenerateResetToken(email string, ttl time.Duration) (*models.PasswordResetToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(b)

	if ttl == 0 {
		ttl = 15 * time.Minute
	}
	expires := time.Now().Add(ttl)

	_, _ = s.DB.Exec("UPDATE password_reset_tokens SET used = true WHERE email = $1 AND used = false", email)

	_, err := s.DB.Exec(`
        INSERT INTO password_reset_tokens (email, token, expires_at, used)
        VALUES ($1, $2, $3, false)
    `, email, token, expires)
	if err != nil {
		return nil, err
	}

	return &models.PasswordResetToken{
		Email:     email,
		Token:     token,
		ExpiresAt: expires,
		Used:      false,
		CreatedAt: time.Now(),
	}, nil
}

func (s *TokenService) ValidateResetToken(token string) (*models.PasswordResetToken, error) {
	var prt models.PasswordResetToken

	err := s.DB.QueryRow(`
        SELECT id, email, token, expires_at, used, created_at
        FROM password_reset_tokens
        WHERE token = $1
    `, token).Scan(&prt.ID, &prt.Email, &prt.Token, &prt.ExpiresAt, &prt.Used, &prt.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("token not found")
		}
		return nil, err
	}

	if prt.Used {
		return nil, errors.New("token already used")
	}
	if time.Now().After(prt.ExpiresAt) {
		return nil, errors.New("token expired")
	}

	_, err = s.DB.Exec(`UPDATE password_reset_tokens SET used=true WHERE id=$1`, prt.ID)
	if err != nil {
		return nil, err
	}

	prt.Used = true
	return &prt, nil
}

// GenerateAccessToken issues a JWT with claims
func (s *TokenService) GenerateAccessToken(userID int64, email string) (string, error) {
	if s.TokenExpiry == 0 {
		s.TokenExpiry = 15 * time.Minute
	}
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(s.TokenExpiry).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.JWTSecret)
}

func (s *TokenService) ValidateAccessToken(tokenStr string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.JWTSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return &claims, nil
}

func (s *TokenService) GenerateRefreshToken(userID int64, email string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *TokenService) ValidateRefreshToken(token string) (*models.SessionToken, error) {
	var st models.SessionToken

	err := s.DB.QueryRow(`
        SELECT st.id, st.user_id, st.device_id, st.refresh_token, st.expires_at, st.created_at
        FROM session_tokens st
        WHERE st.refresh_token = $1
    `, token).Scan(&st.ID, &st.UserID, &st.DeviceID, &st.RefreshToken, &st.ExpiresAt, &st.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("refresh token not found")
		}
		return nil, err
	}

	if time.Now().After(st.ExpiresAt) {
		return nil, errors.New("refresh token expired")
	}

	return &st, nil
}

// ValidateOTP checks a TOTP code against the user's secret
func (s *TokenService) ValidateOTP(userID int64, code string) bool {
	var secret string
	err := s.DB.QueryRow("SELECT mfa_secret FROM users WHERE id=$1", userID).Scan(&secret)
	if err != nil || secret == "" {
		return false
	}
	// Validate TOTP code with 30s step window
	return totp.Validate(code, secret)
}
