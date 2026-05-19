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
	ErrorInvalidFrequencyValue = errors.New("invalid frequency value")
	ErrorAddingProduct         = errors.New("unable to add new product")
	ErrorProductAlreadyExists  = errors.New("product is already being tracked")
	ErrorProductNotFound       = errors.New("product not found")
	ErrorNoJobsFound           = errors.New("no jobs found")
)

type Service interface {
	RegisterProduct(ctx context.Context, product models.Product) error
	GetJobsToRun(ctx context.Context) ([]models.Job, error)
	UpdateNextRunningTime(ctx context.Context, jobs []models.Job) error
	GetSoonestJob(ctx context.Context) (*models.Job, error)
	SaveProductsHistory(ctx context.Context, products []models.ScrapedProduct) error
	GetProductHistory(ctx context.Context, productName string) ([]models.ScrapedProduct, error)
	ListProducts(ctx context.Context) ([]models.Job, error)
	DeleteProduct(ctx context.Context, productName string) error
}

type ScraperService struct {
	scrapperRepository repository.Repository
	wakeUpChanel       chan struct{}
}

func NewScraperService(scrapperRepository repository.Repository, wakeupChanel chan struct{}) *ScraperService {
	return &ScraperService{
		scrapperRepository: scrapperRepository,
		wakeUpChanel:       wakeupChanel,
	}
}

func (s *ScraperService) RegisterProduct(ctx context.Context, product models.Product) error {
	log.Printf("Registering product %q with frequency %q", product.Name, product.Frequency)
	nextRunningTime := s.frequencyHandler(product.Frequency)

	if nextRunningTime == -1 {
		return ErrorInvalidFrequencyValue
	}

	newJob := models.Job{
		ProductName: product.Name,
		Frequency:   product.Frequency,
		TimeToRun:   nextRunningTime,
	}

	err := s.scrapperRepository.InsertNewJob(ctx, newJob)
	if err != nil {
		if errors.Is(err, repository.ErrorProductAlreadyExists) {
			return ErrorProductAlreadyExists
		}
		return ErrorAddingProduct
	}
	s.sendWakeUpCallToOrchestrator()

	return nil
}

func (s *ScraperService) GetJobsToRun(ctx context.Context) ([]models.Job, error) {
	log.Printf("Extracting jobs to run")
	return s.scrapperRepository.GetJobAvailableToRun(ctx, time.Now().Unix())
}

func (s *ScraperService) UpdateNextRunningTime(ctx context.Context, jobs []models.Job) error {
	log.Printf("Updating next running time for %d jobs", len(jobs))
	for i := range jobs {
		jobs[i].TimeToRun = s.frequencyHandler(jobs[i].Frequency)
	}
	return s.scrapperRepository.BatchJobRunningTimeUpdate(ctx, jobs)
}

func (s *ScraperService) frequencyHandler(frequency string) int64 {
	val, ok := FrequencyHandler[frequency]
	if !ok {
		log.Printf("Invalid frequency value: %q", frequency)
		return -1
	}
	return val()
}

func (s *ScraperService) GetSoonestJob(ctx context.Context) (*models.Job, error) {
	job, err := s.scrapperRepository.GetSoonestJob(ctx)
	if errors.Is(err, repository.ErrorNoJobsFound) {
		return nil, ErrorNoJobsFound
	}
	return job, err
}

func (s *ScraperService) SaveProductsHistory(ctx context.Context, products []models.ScrapedProduct) error {
	log.Printf("Saving products history, count: %d", len(products))
	now := time.Now().Unix()
	for i := range products {
		products[i].Time = now
	}
	return s.scrapperRepository.InsertProductHistory(ctx, products)
}

func (s *ScraperService) GetProductHistory(ctx context.Context, productName string) ([]models.ScrapedProduct, error) {
	log.Printf("Fetching price history for %q", productName)
	return s.scrapperRepository.GetProductHistory(ctx, productName)
}

func (s *ScraperService) ListProducts(ctx context.Context) ([]models.Job, error) {
	log.Printf("Listing all tracked products")
	return s.scrapperRepository.GetAllJobs(ctx)
}

func (s *ScraperService) DeleteProduct(ctx context.Context, productName string) error {
	log.Printf("Deleting product %q", productName)
	err := s.scrapperRepository.DeleteJob(ctx, productName)
	if errors.Is(err, repository.ErrorJobNotFound) {
		return ErrorProductNotFound
	}
	return err
}

func (s *ScraperService) sendWakeUpCallToOrchestrator() {
	log.Printf("Sending wake up call to orchestrator")
	select {
	case s.wakeUpChanel <- struct{}{}:
	default:
	}
}
