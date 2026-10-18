package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	FrontendOrigin string

	// TimerPingTimeout is how long a running timer may go without a ping before the
	// sweeper stops it. SweepInterval is how often the sweeper looks. Keep the timeout
	// comfortably longer than the browser's ping interval: browsers throttle timers in
	// hidden tabs (down to roughly once a minute), so a short timeout would stop timers
	// belonging to tabs that are open but in the background.
	TimerPingTimeout time.Duration
	SweepInterval    time.Duration
}

func Load() Config {
	return Config{
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      mustEnv("DATABASE_URL"),
		FrontendOrigin:   getEnv("FRONTEND_ORIGIN", "http://localhost:3001"),
		TimerPingTimeout: getDuration("TIMER_PING_TIMEOUT", 2*time.Minute),
		SweepInterval:    getDuration("SWEEP_INTERVAL", 30*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// getDuration reads an env var like "90s" or "2m". A value that does not parse falls back
// instead of crashing the server, so one typo cannot take the API down.
func getDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("config: %s=%q is not a duration (%v), using %s", key, value, err, fallback)
		return fallback
	}
	return parsed
}

func mustEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		log.Fatalf("missing required env var %s", key)
	}
	return value
}
