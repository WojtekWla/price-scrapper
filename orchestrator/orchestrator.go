package orchestrator

import (
	"context"
	"errors"
	"log"
	"price-scrapper/discord"
	"price-scrapper/llm"
	"price-scrapper/orchestrator/scraper"
	"price-scrapper/service"
	"sync/atomic"
	"time"
)

const maxConcurrentJobs = 5

type ScrapJobOrchestrator struct {
	scrapService    service.Service
	geminiSvc       *llm.GeminiService
	discordNotifier *discord.Notifier
	wakeUpChanel    chan struct{}
	sem             chan struct{}
	runningJobs     atomic.Int32
}

func NewOrchestrator(svc service.Service, geminiSvc *llm.GeminiService, discordNotifier *discord.Notifier, wakeUpChanel chan struct{}) *ScrapJobOrchestrator {
	return &ScrapJobOrchestrator{
		scrapService:    svc,
		geminiSvc:       geminiSvc,
		discordNotifier: discordNotifier,
		wakeUpChanel:    wakeUpChanel,
		sem:             make(chan struct{}, maxConcurrentJobs),
	}
}

func (o *ScrapJobOrchestrator) RunOrchestrator(ctx context.Context) {
	for {
		jobs, err := o.scrapService.GetJobsToRun(ctx)

		if err != nil {
			if err != nil {
				log.Printf("Unable to extract jobs: %v, retrying in 5 seconds...", err)
			}
			if o.waitWithWakeup(ctx, 5*time.Second) {
				return
			}
			continue
		}

		if len(jobs) == 0 {
			soonest, err := o.scrapService.GetSoonestJob(ctx)
			if errors.Is(err, service.ErrorNoJobsFound) {
				log.Printf("No jobs in queue, waiting for new job")
				if o.waitForWakeup(ctx) {
					return
				}
				continue
			}
			if err != nil {
				log.Printf("Unable to get soonest job: %v, retrying in 5 seconds...", err)
				if o.waitWithWakeup(ctx, 5*time.Second) {
					return
				}
				continue
			}
			waitDuration := time.Until(time.Unix(soonest.TimeToRun, 0))
			if waitDuration < 0 {
				waitDuration = 0
			}
			log.Printf("No jobs due yet, next job %q in %v", soonest.ProductName, waitDuration)
			if o.waitWithWakeup(ctx, waitDuration) {
				return
			}
			continue
		}

		for _, job := range jobs {
			o.sem <- struct{}{}
			go func() {
				running := o.runningJobs.Add(1)
				log.Printf("Job started for %q, running: %d/%d", job.ProductName, running, maxConcurrentJobs)
				defer func() {
					<-o.sem
					running := o.runningJobs.Add(-1)
					log.Printf("Job finished for %q, running: %d/%d", job.ProductName, running, maxConcurrentJobs)
				}()

				s, err := scraper.New()
				if err != nil {
					log.Printf("Failed to create scraper: %v", err)
					return
				}
				defer s.Close()

				data, err := s.SearchAndScrapeProduct(ctx, job.ProductName)
				if err != nil {
					log.Printf("Failed to search and scrape %q: %v", job.ProductName, err)
					return
				}

				log.Printf("Sending %d bytes to Gemini for %q", len(data), job.ProductName)

				products, err := o.geminiSvc.ExtractProducts(ctx, data)
				if err != nil {
					log.Printf("Gemini extraction failed for %q: %v", job.ProductName, err)
					return
				}

				log.Printf("Gemini extracted %d products for %q", len(products), job.ProductName)

				for i := range products {
					products[i].SearchTerm = job.ProductName
				}

				if err := o.scrapService.SaveProductsHistory(ctx, products); err != nil {
					log.Printf("Failed to save product history for %q: %v", job.ProductName, err)
					return
				}

				if o.discordNotifier != nil && len(products) > 0 {
					if err := o.discordNotifier.NotifyProducts(job.ProductName, products); err != nil {
						log.Printf("Discord notification failed for %q: %v", job.ProductName, err)
					}
				}
			}()
		}

		if err := o.scrapService.UpdateNextRunningTime(ctx, jobs); err != nil {
			log.Printf("Unable to update next running time: %v", err)
		}

		log.Printf("Next running time updated, calculating wait duration")

		soonest, err := o.scrapService.GetSoonestJob(ctx)
		waitDuration := 30 * time.Second

		if err == nil && soonest != nil {
			waitDuration = time.Until(time.Unix(soonest.TimeToRun, 0))
			if waitDuration < 0 {
				waitDuration = 0
			}
		}

		log.Printf("Next job extraction in %v", waitDuration)

		if o.waitWithWakeup(ctx, waitDuration) {
			return
		}
	}
}

func (o *ScrapJobOrchestrator) waitForWakeup(ctx context.Context) bool {
	select {
	case <-o.wakeUpChanel:
		return false
	case <-ctx.Done():
		return true
	}
}

func (o *ScrapJobOrchestrator) waitWithWakeup(ctx context.Context, d time.Duration) bool {
	if d == 0 {
		return false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-o.wakeUpChanel:
		return false
	case <-ctx.Done():
		return true
	}
}
