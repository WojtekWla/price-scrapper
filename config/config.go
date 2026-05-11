package config

import "os"

type DatabaseConfig struct {
	User          string
	Password      string
	DbName        string
	DbAddress     string
	DbPort        string
	MigrationFile string
}

type Config struct {
	DB           DatabaseConfig
	GeminiAPIKey string
}

func InitializeConfigs() Config {
	return Config{
		DB: DatabaseConfig{
			User:          os.Getenv("DB_USER"),
			Password:      os.Getenv("DB_PASSWORD"),
			DbName:        os.Getenv("DB_NAME"),
			DbAddress:     os.Getenv("DB_ADDRESS"),
			DbPort:        os.Getenv("DB_PORT"),
			MigrationFile: os.Getenv("MIGRATIONS"),
		},
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
	}
}
