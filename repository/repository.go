package repository

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertNewJob = `INSERT INTO scrap_job(product_name, next_running_time) VALUES(@prodName, @nextRunningTime)`
)

var (
	ErrorInsertingNewJob = errors.New("Unable to insert new job")
)

type Repository interface {
	InsertNewJob(ctx context.Context, product_name string, next_running_time int64) error
}

type ScrapperRepository struct {
	dbPool *pgxpool.Pool
}

func NewScrapperRepository(dbPool *pgxpool.Pool) *ScrapperRepository {
	return &ScrapperRepository{
		dbPool: dbPool,
	}
}

func (r *ScrapperRepository) InsertNewJob(ctx context.Context, product_name string, next_running_time int64) error {
	args := pgx.NamedArgs{
		"prodName":        product_name,
		"nextRunningTime": next_running_time,
	}

	_, err := r.dbPool.Exec(ctx, insertNewJob, args)

	if err != nil {
		log.Printf("Error executing query, %v", err)
		return ErrorInsertingNewJob
	}

	return nil
}
