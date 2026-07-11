package config

import (
	"log"
	"os"
)

// Config holds everything the service reads from the environment.
// It grows as we add layers (Kafka, Redis, S3) — for now DB + HTTP/auth.
type Config struct {
	ServerPort     string
	DatabaseURL    string
	JWKSUrl        string
	AllowedOrigins string
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8082"),
		DatabaseURL:    dbURL,
		JWKSUrl:        getEnv("JWKS_URL", "http://localhost:8180/realms/appraisal/protocol/openid-connect/certs"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "*"),
	}
}

// getEnv returns the env var or a fallback when it is unset/empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
