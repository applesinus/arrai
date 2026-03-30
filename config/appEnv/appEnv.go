package appEnv

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type AppEnv struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *AppEnv {
	if err := godotenv.Load(); err != nil {
		logger.Error("Failed to load .env file",
			"error", err,
		)
		panic("Failed to load .env file, check logs for more details")
	}

	return &AppEnv{logger: logger}
}

func (s *AppEnv) GetOrDefault(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		s.logger.Warn("Environment variable is not set. Using default value",
			"key", key,
			"default_value", defaultValue,
		)
		return defaultValue
	}

	return value
}

func (s *AppEnv) GetIntOrDefault(key string, defaultValue int) int {
	value := s.GetOrDefault(key, fmt.Sprintf("%d", defaultValue))
	intValue, err := strconv.Atoi(value)
	if err != nil {
		s.logger.Warn("Environment variable is not a valid integer. Using default value",
			"key", key,
			"default_value", defaultValue,
		)

		return defaultValue
	}

	return intValue
}

func (s *AppEnv) GetBoolOrDefault(key string, defaultValue bool) bool {
	value := s.GetOrDefault(key, fmt.Sprintf("%t", defaultValue))
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		s.logger.Warn("Environment variable is not a valid boolean. Using default value",
			"key", key,
			"default_value", defaultValue,
		)

		return defaultValue
	}

	return boolValue
}

func (s *AppEnv) MustGet(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		s.logger.Error("Environment variable is not set",
			"key", key,
		)
		panic("Environment variable is not set, check logs for more details")
	}

	return value
}

func (s *AppEnv) MustGetInt(key string) int {
	value := s.MustGet(key)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		s.logger.Error("Environment variable is not a valid integer",
			"key", key,
		)
		panic("Environment variable is not a valid integer, check logs for more details")
	}

	return intValue
}

func (s *AppEnv) MustGetBool(key string) bool {
	value := s.MustGet(key)
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		s.logger.Error("Environment variable is not a valid boolean",
			"key", key,
		)
		panic("Environment variable is not a valid boolean, check logs for more details")
	}

	return boolValue
}
