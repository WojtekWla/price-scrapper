package orchestrator

import (
	"context"
	"log"
	"os"
	"price-scrapper/llm"
	"price-scrapper/orchestrator/scraper"
	"price-scrapper/service"
	"time"
)

type ScrapJobOrchestrator struct {
	scrapService service.Service
	geminiSvc    *llm.GeminiService
}

func NewOrchestrator(service service.Service, geminiSvc *llm.GeminiService) *ScrapJobOrchestrator {
	return &ScrapJobOrchestrator{
		scrapService: service,
		geminiSvc:    geminiSvc,
	}
}

func (o *ScrapJobOrchestrator) RunOrchestrator(ctx context.Context) {
	for {
		jobs, err := o.scrapService.GetJobsToRun(ctx)

		if err != nil || len(jobs) == 0 {
			log.Printf("Unable to extract jobs, retry in 5 seconds...")
			if o.wait(ctx, 5*time.Second) {
				return
			}
			continue
		}

		for _, job := range jobs {
			go func() {
				s, err := scraper.New()
				if err != nil {
					log.Printf("failed to create scraper: %v", err)
					return
				}
				defer s.Close()

				data, err := s.SearchAndScrapeProduct(ctx, job.ProductName)
				if err != nil {
					log.Printf("failed to search and scrape: %v", err)
					return
				}

				if err := os.WriteFile("tmp.txt", []byte(data), 0644); err != nil {
					log.Printf("failed to write tmp.txt: %v", err)
				}

				log.Printf("scraped %d bytes, written to tmp.txt", len(data))
				log.Printf("sending %d bytes to Gemini for %q", len(data), job.ProductName)

				result, err := o.geminiSvc.ExtractProducts(ctx, data)
				if err != nil {
					log.Printf("Gemini extraction failed: %v", err)
					return
				}

				log.Printf("Gemini response received for %q", job.ProductName)
				log.Printf("Following products were extracted for product: %s:\n%s", job.ProductName, result)
			}()
		}

		err = o.scrapService.UpdateNextRunningTime(ctx, jobs)
		if err != nil {
			log.Printf("Unable to update next running time")
		}

		log.Printf("Next running time updated, extracting soonest job to calculate wait duration")

		soonest, err := o.scrapService.GetSoonestJob(ctx)
		waitDuration := 30 * time.Second

		if err == nil && soonest != nil {
			waitDuration = time.Until(time.Unix(soonest.TimeToRun, 0))
			if waitDuration < 0 {
				waitDuration = 0
			}
		}

		log.Printf("Next job extraction in %v", waitDuration)

		if o.wait(ctx, waitDuration) {
			return
		}
	}
}

func (o *ScrapJobOrchestrator) wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-ctx.Done():
		return true
	}
}
