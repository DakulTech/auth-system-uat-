package repository

import (
	"auth/internal/models"
	"database/sql"
	"errors"
)

type PostgresUserRepository struct {
	DB *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{DB: db}
}

func (r *PostgresUserRepository) FindByEmail(email string) (*models.User, error) {
	user := &models.User{}
	row := r.DB.QueryRow("SELECT id, email, password, verified FROM users WHERE email=$1", email)
	err := row.Scan(&user.ID, &user.Email, &user.Password, &user.Verified)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *PostgresUserRepository) Create(user *models.User) error {
	err := r.DB.QueryRow(
		"INSERT INTO users (email, password, verified) VALUES ($1, $2, $3) RETURNING id",
		user.Email, user.Password, user.Verified,
	).Scan(&user.ID)
	if err != nil {
		return errors.New("failed to create user: " + err.Error())
	}
	return nil
}
