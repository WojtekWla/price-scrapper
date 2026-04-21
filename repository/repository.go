package repository

import (
	"context"
	"errors"
	"log"
	"price-scrapper/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertNewJob         = `INSERT INTO scrap_job(product_name, next_running_time, frequency) VALUES(@prodName, @nextRunningTime, @frequency)`
	getJobsToRun         = `SELECT id, product_name, frequency, next_running_time FROM scrap_job WHERE next_running_time < @now`
	updateJobRunningTime = `UPDATE scrap_job SET next_running_time = $1 WHERE id = $2`
	getSoonestJob        = `SELECT id, product_name, frequency, next_running_time FROM scrap_job ORDER BY next_running_time ASC LIMIT 1`
)

var (
	ErrorInsertingNewJob       = errors.New("Unable to insert new job")
	ErrorExtractingJobsToRun   = errors.New("Unable to extract jobs")
	ErrorUpdateJobsRunningTime = errors.New("Unable to update jobs running time")
	ErrorExtractingSoonestJob  = errors.New("Unable to extract soonest job")
)

type Repository interface {
	InsertNewJob(ctx context.Context, newJob models.Job) error
	GetJobAvailableToRun(ctx context.Context, current_time int64) ([]models.Job, error)
	BatchJobRunningTimeUpdate(ctx context.Context, jobs []models.Job) error
	GetSoonestJob(ctx context.Context) (*models.Job, error)
}

type ScrapperRepository struct {
	dbPool *pgxpool.Pool
}

func NewScrapperRepository(dbPool *pgxpool.Pool) *ScrapperRepository {
	return &ScrapperRepository{
		dbPool: dbPool,
	}
}

func (r *ScrapperRepository) InsertNewJob(ctx context.Context, newJob models.Job) error {
	args := pgx.NamedArgs{
		"prodName":        newJob.ProductName,
		"nextRunningTime": newJob.TimeToRun,
		"frequency":       newJob.Frequency,
	}

	_, err := r.dbPool.Exec(ctx, insertNewJob, args)

	if err != nil {
		log.Printf("Error executing query, %v", err)
		return ErrorInsertingNewJob
	}

	return nil
}

func (r *ScrapperRepository) GetJobAvailableToRun(ctx context.Context, current_time int64) ([]models.Job, error) {
	args := pgx.NamedArgs{
		"now": current_time,
	}

	rows, err := r.dbPool.Query(ctx, getJobsToRun, args)
	if err != nil {
		log.Printf("Error executing query, %v", err)
		return nil, ErrorExtractingJobsToRun
	}

	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job

		if err := rows.Scan(&j.Id, &j.ProductName, &j.Frequency, &j.TimeToRun); err != nil {
			log.Printf("Error extracting job, %v", err)
			return nil, ErrorExtractingJobsToRun
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *ScrapperRepository) BatchJobRunningTimeUpdate(ctx context.Context, jobs []models.Job) error {
	tx, err := r.dbPool.Begin(ctx)

	if err != nil {
		log.Printf("Error starting transaction, %v", err)
		return ErrorUpdateJobsRunningTime
	}

	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, job := range jobs {
		batch.Queue(updateJobRunningTime, job.TimeToRun, job.Id)
	}

	batchUpdate := tx.SendBatch(ctx, batch)

	for i := 0; i < len(jobs); i++ {
		_, err := batchUpdate.Exec()
		if err != nil {
			batchUpdate.Close()
			log.Printf("Error in batch at item %d: %v", i, err)
			return ErrorUpdateJobsRunningTime
		}
	}

	if err := batchUpdate.Close(); err != nil {
		log.Printf("batch execution failed: %v", err)
		return ErrorUpdateJobsRunningTime
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error commiting transaction, %v", err)
		return ErrorUpdateJobsRunningTime
	}

	return nil
}

func (r *ScrapperRepository) GetSoonestJob(ctx context.Context) (*models.Job, error) {
	var job models.Job
	row := r.dbPool.QueryRow(ctx, getSoonestJob)

	if err := row.Scan(&job.Id, &job.ProductName, &job.Frequency, &job.TimeToRun); err != nil {
		log.Printf("Error extracting soonest job, %v", err)
		return nil, ErrorExtractingSoonestJob
	}

	return &job, nil
}
