package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"price-scrapper/config"
	"price-scrapper/db"
	"price-scrapper/llm"
	"price-scrapper/notifier"
	"price-scrapper/orchestrator"
	pb "price-scrapper/proto_gen"
	"price-scrapper/repository"
	"price-scrapper/service"
	"syscall"

	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting scraping app")

	conf, err := config.InitializeConfigs()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := db.RunMigrations(ctx, conf.DB); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	dbPool, err := db.CreateConnection(ctx, conf.DB)
	if err != nil {
		log.Fatalf("Unable to create db connection: %v", err)
	}
	defer dbPool.Close()
	log.Println("Connected to database")

	geminiSvc, err := llm.NewGeminiService(ctx, conf.GeminiAPIKey, conf.GeminiRPM)
	if err != nil {
		log.Fatalf("Failed to create Gemini service: %v", err)
	}
	defer geminiSvc.Close()

	communicationNotifier := notifier.NewNotifier(conf.NotifierConfig)

	wakeUpChanel := make(chan struct{}, 1)
	repo := repository.NewScrapperRepository(dbPool)
	svc := service.NewScraperService(repo, wakeUpChanel)
	orch := orchestrator.NewOrchestrator(svc, geminiSvc, communicationNotifier, wakeUpChanel)

	go orch.RunOrchestrator(ctx)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	pb.RegisterScraperServer(s, &Server{
		ScrapperService: svc,
	})

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
		<-quit
		log.Println("Shutting down...")
		cancel()
		s.GracefulStop()
	}()

	log.Printf("Server listening on %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
