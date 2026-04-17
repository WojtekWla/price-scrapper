package service

import (
	"context"
	"errors"
	"log"
	"price-scrapper/repository"
)

var (
	ErrorInvalidFrequencyValue = errors.New("Invalid frequency value")
	ErrorAddingProduct         = errors.New("Unable to add new product")
)

type Service interface {
	RegisterProduct(ctx context.Context, product Product) error
}

type ScraperService struct {
	scrapperRepository repository.Repository
}

func NewScraperService(scrapperRepository repository.Repository) *ScraperService {
	return &ScraperService{
		scrapperRepository: scrapperRepository,
	}
}

func (s *ScraperService) RegisterProduct(ctx context.Context, product Product) error {
	log.Printf("Registering users product %s, with frequncy %s", product.Name, product.Frequency)
	next_running_time := s.handleFrequency(product.Frequency)

	if next_running_time == -1 {
		return ErrorInvalidFrequencyValue
	}

	err := s.scrapperRepository.InsertNewJob(ctx, product.Name, next_running_time)
	if err != nil {
		return ErrorAddingProduct
	}

	return nil
}

func (s *ScraperService) handleFrequency(frequency string) int64 {
	val, ok := FrequencyHandler[frequency]

	if !ok {
		log.Printf("Invalid frequency value")
		return -1
	}

	return val()
}
