package db

import (
	"context"
	"fmt"
	"log"
	"price-scrapper/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(ctx context.Context, dbConf config.DatabaseConfig) error {
	connectionString := fmt.Sprintf("pgx5://%s:%s@%s:%s/%s?sslmode=disable",
		dbConf.User,
		dbConf.Password,
		dbConf.DbAddress,
		dbConf.DbPort,
		dbConf.DbName,
	)

	m, err := migrate.New(dbConf.MigrationFile, connectionString)
	if err != nil {
		log.Fatalf("Critical Migration Error: %v", err)
		return err
	}

	log.Println("Running migrations...")
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("No new migrations to apply.")
			return nil
		}
		log.Fatalf("Migrations failed")
		return err
	}

	log.Println("Migrations applied successfully")
	return nil
}
