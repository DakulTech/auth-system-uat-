package config

import "sync"

type WorkerConfig struct {
	DBConn    string
	RedisAddr string
	RedisPass string
	SMTPHost  string
	SMTPPort  string
	SMTPUser  string
	SMTPPass  string
}

var (
	workerCfg  *WorkerConfig
	workerOnce sync.Once
)

func LoadWorkerConfig() *WorkerConfig {
	workerOnce.Do(func() {
		workerCfg = &WorkerConfig{
			DBConn:    getEnv("DB_CONN", "postgres://postgres:password@localhost:5432/authdb?sslmode=disable"),
			RedisAddr: getEnv("REDIS_ADDR", "localhost:6379"),
			RedisPass: getEnv("REDIS_PASSWORD", ""),
			SMTPHost:  getEnv("SMTP_HOST", "smtp.gmail.com"),
			SMTPPort:  getEnv("SMTP_PORT", "587"),
			SMTPUser:  getEnv("SMTP_USER", "your-email@gmail.com"),
			SMTPPass:  getEnv("SMTP_PASS", ""),
		}
	})
	return workerCfg
}
