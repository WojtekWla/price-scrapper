package db

import (
	"context"
	"fmt"

	"price-scrapper/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateConnection(ctx context.Context, dbConf config.DatabaseConfig) (*pgxpool.Pool, error) {
	connectionString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbConf.User,
		dbConf.Password,
		dbConf.DbAddress,
		dbConf.DbPort,
		dbConf.DbName,
	)
	return pgxpool.New(ctx, connectionString)
}
