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

func InitializeConfigs() DatabaseConfig {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbAddress := os.Getenv("DB_ADDRESS")
	dbPort := os.Getenv("DB_PORT")
	migrations := os.Getenv("MIGRATIONS")

	return DatabaseConfig{
		User:          user,
		Password:      password,
		DbName:        dbName,
		DbAddress:     dbAddress,
		DbPort:        dbPort,
		MigrationFile: migrations,
	}
}
