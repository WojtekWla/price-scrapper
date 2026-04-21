package orchestrator

import (
	"context"
	"log"
	"price-scrapper/service"
	"time"
)

type ScrapJobOrchestrator struct {
	scrapService service.Service
}

func NewOrchestrator(service service.Service) *ScrapJobOrchestrator {
	return &ScrapJobOrchestrator{
		scrapService: service,
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
			log.Printf("Executing job: %s, %s, %d", job.Id, job.ProductName, job.TimeToRun)
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
