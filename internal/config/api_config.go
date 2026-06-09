package config

import "sync"

type APIConfig struct {
	DBConn     string
	RedisAddr  string
	RedisPass  string
	ServerPort string
}

var (
	apiCfg  *APIConfig
	apiOnce sync.Once
)

func LoadAPIConfig() *APIConfig {
	apiOnce.Do(func() {
		apiCfg = &APIConfig{
			DBConn:     getEnv("DB_CONN", "postgres://postgres:password@localhost:5432/authdb?sslmode=disable"),
			RedisAddr:  getEnv("REDIS_ADDR", "localhost:6379"),
			RedisPass:  getEnv("REDIS_PASSWORD", ""),
			ServerPort: getEnv("SERVER_PORT", "8080"),
		}
	})
	return apiCfg
}
