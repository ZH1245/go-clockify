package config

import (
	"log"
	"os"
)

type Config struct {
	Port           string
	DatabaseURL    string
	FrontendOrigin string
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    mustEnv("DATABASE_URL"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:3001"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func mustEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return value
}
