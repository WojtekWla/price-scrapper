package repository

import (
	"context"
	"errors"
	"log"
	"price-scrapper/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insertNewJob         = `INSERT INTO scrap_job(product_name, next_running_time, frequency) VALUES(@prodName, @nextRunningTime, @frequency)`
	getJobsToRun         = `SELECT id, product_name, frequency, next_running_time FROM scrap_job WHERE next_running_time < @now`
	updateJobRunningTime = `UPDATE scrap_job SET next_running_time = $1 WHERE id = $2`
	getSoonestJob        = `SELECT id, product_name, frequency, next_running_time FROM scrap_job ORDER BY next_running_time ASC LIMIT 1`
	insertProductHistory = `INSERT INTO product_history(product_name, price, link, scraped_at, search_term) VALUES($1, $2, $3, $4, $5)`
	getProductHistory    = `SELECT product_name, price, link, scraped_at FROM product_history WHERE search_term = $1 ORDER BY scraped_at DESC`
	getAllJobs            = `SELECT id, product_name, frequency, next_running_time FROM scrap_job ORDER BY created_at`
	deleteJob            = `DELETE FROM scrap_job WHERE product_name = $1`
)

// uniqueViolationCode is the PostgreSQL error code for unique constraint violations.
const uniqueViolationCode = "23505"

var (
	ErrorInsertingNewJob         = errors.New("unable to insert new job")
	ErrorProductAlreadyExists    = errors.New("product is already being tracked")
	ErrorExtractingJobsToRun     = errors.New("unable to extract jobs")
	ErrorUpdateJobsRunningTime   = errors.New("unable to update jobs running time")
	ErrorExtractingSoonestJob    = errors.New("unable to extract soonest job")
	ErrorInsertingProductHistory = errors.New("unable to insert product history")
	ErrorExtractingProductHistory = errors.New("unable to extract product history")
	ErrorExtractingAllJobs       = errors.New("unable to extract all jobs")
	ErrorDeletingJob             = errors.New("unable to delete job")
	ErrorJobNotFound             = errors.New("product not found")
)

type Repository interface {
	InsertNewJob(ctx context.Context, newJob models.Job) error
	GetJobAvailableToRun(ctx context.Context, current_time int64) ([]models.Job, error)
	BatchJobRunningTimeUpdate(ctx context.Context, jobs []models.Job) error
	GetSoonestJob(ctx context.Context) (*models.Job, error)
	InsertProductHistory(ctx context.Context, products []models.ScrapedProduct) error
	GetProductHistory(ctx context.Context, searchTerm string) ([]models.ScrapedProduct, error)
	GetAllJobs(ctx context.Context) ([]models.Job, error)
	DeleteJob(ctx context.Context, productName string) error
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return ErrorProductAlreadyExists
		}
		log.Printf("Error executing query: %v", err)
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
		log.Printf("Error executing query: %v", err)
		return nil, ErrorExtractingJobsToRun
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(&j.Id, &j.ProductName, &j.Frequency, &j.TimeToRun); err != nil {
			log.Printf("Error scanning job: %v", err)
			return nil, ErrorExtractingJobsToRun
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *ScrapperRepository) BatchJobRunningTimeUpdate(ctx context.Context, jobs []models.Job) error {
	tx, err := r.dbPool.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
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
		log.Printf("Batch execution failed: %v", err)
		return ErrorUpdateJobsRunningTime
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return ErrorUpdateJobsRunningTime
	}

	return nil
}

func (r *ScrapperRepository) GetSoonestJob(ctx context.Context) (*models.Job, error) {
	var job models.Job
	row := r.dbPool.QueryRow(ctx, getSoonestJob)

	if err := row.Scan(&job.Id, &job.ProductName, &job.Frequency, &job.TimeToRun); err != nil {
		log.Printf("Error extracting soonest job: %v", err)
		return nil, ErrorExtractingSoonestJob
	}

	return &job, nil
}

func (r *ScrapperRepository) InsertProductHistory(ctx context.Context, products []models.ScrapedProduct) error {
	tx, err := r.dbPool.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return ErrorInsertingProductHistory
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, p := range products {
		batch.Queue(insertProductHistory, p.Name, p.Price, p.Link, p.Time, p.SearchTerm)
	}

	br := tx.SendBatch(ctx, batch)

	for i := 0; i < len(products); i++ {
		if _, err := br.Exec(); err != nil {
			br.Close()
			log.Printf("Error in batch at item %d: %v", i, err)
			return ErrorInsertingProductHistory
		}
	}

	if err := br.Close(); err != nil {
		log.Printf("Batch execution failed: %v", err)
		return ErrorInsertingProductHistory
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return ErrorInsertingProductHistory
	}

	return nil
}

func (r *ScrapperRepository) GetProductHistory(ctx context.Context, searchTerm string) ([]models.ScrapedProduct, error) {
	rows, err := r.dbPool.Query(ctx, getProductHistory, searchTerm)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, ErrorExtractingProductHistory
	}
	defer rows.Close()

	var products []models.ScrapedProduct
	for rows.Next() {
		var p models.ScrapedProduct
		if err := rows.Scan(&p.Name, &p.Price, &p.Link, &p.Time); err != nil {
			log.Printf("Error scanning product history: %v", err)
			return nil, ErrorExtractingProductHistory
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *ScrapperRepository) GetAllJobs(ctx context.Context) ([]models.Job, error) {
	rows, err := r.dbPool.Query(ctx, getAllJobs)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, ErrorExtractingAllJobs
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		if err := rows.Scan(&j.Id, &j.ProductName, &j.Frequency, &j.TimeToRun); err != nil {
			log.Printf("Error scanning job: %v", err)
			return nil, ErrorExtractingAllJobs
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *ScrapperRepository) DeleteJob(ctx context.Context, productName string) error {
	result, err := r.dbPool.Exec(ctx, deleteJob, productName)
	if err != nil {
		log.Printf("Error executing delete: %v", err)
		return ErrorDeletingJob
	}

	if result.RowsAffected() == 0 {
		return ErrorJobNotFound
	}

	return nil
}
