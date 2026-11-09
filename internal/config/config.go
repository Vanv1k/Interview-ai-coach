package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramBotToken string
	DatabaseURL      string
	LLMAPIKey        string
	LLMBaseURL       string
	LLMModel         string
	LLMTimeout       time.Duration
	LLMMaxRetries    int
	MaxAnswerLength  int
	AnswerRateLimit  int
	StartRateLimit   int
	LogLevel         slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		LLMAPIKey:        strings.TrimSpace(os.Getenv("LLM_API_KEY")),
		LLMBaseURL:       env("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMModel:         env("LLM_MODEL", "gpt-4.1-mini"),
		LLMTimeout:       durationEnv("LLM_TIMEOUT", 45*time.Second),
		LLMMaxRetries:    intEnv("LLM_MAX_RETRIES", 2),
		MaxAnswerLength:  intEnv("MAX_ANSWER_LENGTH", 6000),
		AnswerRateLimit:  intEnv("ANSWER_RATE_LIMIT_PER_MINUTE", 6),
		StartRateLimit:   intEnv("START_RATE_LIMIT_PER_HOUR", 10),
		LogLevel:         parseLevel(env("LOG_LEVEL", "info")),
	}
	if cfg.TelegramBotToken == "" {
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.LLMAPIKey == "" {
		return Config{}, errors.New("LLM_API_KEY is required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
