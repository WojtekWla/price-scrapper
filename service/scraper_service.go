package service

import "log"

type Service interface {
	RegisterProduct(product Product) error
}

type ScraperService struct {
}

func NewScraperService() *ScraperService {
	return &ScraperService{}
}

func (ss *ScraperService) RegisterProduct(product Product) error {
	log.Printf("Registering users product %s, with frequncy %s", product.Name, product.Frequency)
	return nil
}
