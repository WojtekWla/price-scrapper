package config

import (
	"fmt"
	"os"
	"strconv"
)

type DatabaseConfig struct {
	User          string
	Password      string
	DbName        string
	DbAddress     string
	DbPort        string
	MigrationFile string
}

type Config struct {
	DB                DatabaseConfig
	GeminiAPIKey      string
	GeminiRPM         int
	DiscordWebhookURL string
}

func InitializeConfigs() (Config, error) {
	required := map[string]string{
		"DB_USER":        os.Getenv("DB_USER"),
		"DB_PASSWORD":    os.Getenv("DB_PASSWORD"),
		"DB_NAME":        os.Getenv("DB_NAME"),
		"DB_ADDRESS":     os.Getenv("DB_ADDRESS"),
		"DB_PORT":        os.Getenv("DB_PORT"),
		"MIGRATIONS":     os.Getenv("MIGRATIONS"),
		"GEMINI_API_KEY": os.Getenv("GEMINI_API_KEY"),
	}

	for key, val := range required {
		if val == "" {
			return Config{}, fmt.Errorf("required environment variable %s is not set", key)
		}
	}

	rpm, err := strconv.Atoi(os.Getenv("GEMINI_RPM"))
	if err != nil || rpm <= 0 {
		rpm = 15
	}

	return Config{
		DB: DatabaseConfig{
			User:          required["DB_USER"],
			Password:      required["DB_PASSWORD"],
			DbName:        required["DB_NAME"],
			DbAddress:     required["DB_ADDRESS"],
			DbPort:        required["DB_PORT"],
			MigrationFile: required["MIGRATIONS"],
		},
		GeminiAPIKey:      required["GEMINI_API_KEY"],
		GeminiRPM:         rpm,
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
	}, nil
}
