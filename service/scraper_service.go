package service

import (
	"context"
	"errors"
	"log"
	"price-scrapper/models"
	"price-scrapper/repository"
	"time"
)

var (
	ErrorInvalidFrequencyValue = errors.New("Invalid frequency value")
	ErrorAddingProduct         = errors.New("Unable to add new product")
)

type Service interface {
	RegisterProduct(ctx context.Context, product models.Product) error
	GetJobsToRun(ctx context.Context) ([]models.Job, error)
	UpdateNextRunningTime(ctx context.Context, jobs []models.Job) error
	GetSoonestJob(ctx context.Context) (*models.Job, error)
}

type ScraperService struct {
	scrapperRepository repository.Repository
}

func NewScraperService(scrapperRepository repository.Repository) *ScraperService {
	return &ScraperService{
		scrapperRepository: scrapperRepository,
	}
}

func (s *ScraperService) RegisterProduct(ctx context.Context, product models.Product) error {
	log.Printf("Registering users product %s, with frequncy %s", product.Name, product.Frequency)
	next_running_time := s.frequencyHandler(product.Frequency)

	if next_running_time == -1 {
		return ErrorInvalidFrequencyValue
	}

	newJob := models.Job{
		ProductName: product.Name,
		Frequency:   product.Frequency,
		TimeToRun:   next_running_time,
	}

	err := s.scrapperRepository.InsertNewJob(ctx, newJob)
	if err != nil {
		return ErrorAddingProduct
	}

	return nil
}

func (s *ScraperService) GetJobsToRun(ctx context.Context) ([]models.Job, error) {
	log.Printf("Extracting jobs to run")
	currentTime := time.Now().Unix()

	return s.scrapperRepository.GetJobAvailableToRun(ctx, currentTime)
}

func (s *ScraperService) UpdateNextRunningTime(ctx context.Context, jobs []models.Job) error {
	log.Printf("Update next running time in %d jobs", len(jobs))
	for i := range jobs {
		jobs[i].TimeToRun = s.frequencyHandler(jobs[i].Frequency)
	}

	return s.scrapperRepository.BatchJobRunningTimeUpdate(ctx, jobs)
}

func (s *ScraperService) frequencyHandler(frequency string) int64 {
	val, ok := FrequencyHandler[frequency]

	if !ok {
		log.Printf("Invalid frequency value")
		return -1
	}

	return val()
}

func (s *ScraperService) GetSoonestJob(ctx context.Context) (*models.Job, error) {
	return s.scrapperRepository.GetSoonestJob(ctx)
}
