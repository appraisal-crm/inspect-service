package config

import (
	"log"
	"os"
	"time"
)

// Config holds everything the service reads from the environment.
// It grows as we add layers (Kafka, Redis, S3) — for now DB + HTTP/auth.
type Config struct {
	ServerPort         string
	DatabaseURL        string
	JWKSUrl            string
	AllowedOrigins     string
	S3Endpoint         string
	S3Bucket           string
	KafkaBrokers       string
	OutboxPollInterval time.Duration
}

func Load() *Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	return &Config{
		ServerPort:         getEnv("SERVER_PORT", "8082"),
		DatabaseURL:        dbURL,
		JWKSUrl:            getEnv("JWKS_URL", "http://localhost:8180/realms/appraisal/protocol/openid-connect/certs"),
		AllowedOrigins:     getEnv("ALLOWED_ORIGINS", "*"),
		S3Endpoint:         getEnv("S3_ENDPOINT", "https://storage.yandexcloud.net"),
		S3Bucket:           getEnv("S3_BUCKET", "appraisal-inspections"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:9092"),
		OutboxPollInterval: getDurationEnv("OUTBOX_POLL_INTERVAL", time.Second),
	}
}

// getDurationEnv parses a Go duration string (e.g. "1s", "500ms"); a bad value
// is a config error worth failing fast on.
func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Fatalf("invalid %s: %v", key, err)
		}
		return d
	}
	return fallback
}

// getEnv returns the env var or a fallback when it is unset/empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
