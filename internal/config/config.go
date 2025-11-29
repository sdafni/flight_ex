package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort         string
	DatabaseDSN        string
	TemporalAddress    string
	ReservationTimeout time.Duration
	PaymentTimeout     time.Duration
	MaxPaymentRetries  int
}

func Load() *Config {
	return &Config{
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		DatabaseDSN:        getEnv("DATABASE_DSN", "booking_user:booking_pass@tcp(localhost:3306)/flight_booking?parseTime=true"),
		TemporalAddress:    getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
		ReservationTimeout: parseDuration(getEnv("RESERVATION_TIMEOUT", "15m")),
		PaymentTimeout:     parseDuration(getEnv("PAYMENT_TIMEOUT", "10s")),
		MaxPaymentRetries:  parseInt(getEnv("MAX_PAYMENT_RETRIES", "3")),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}
