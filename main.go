package main

import (
	"context"
	"log"
	"net"
	"os"
	"price-scrapper/config"
	"price-scrapper/db"
	"price-scrapper/orchestrator"
	pb "price-scrapper/proto_gen"
	"price-scrapper/repository"
	"price-scrapper/service"

	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting scraping app")
	ctx := context.Background()
	dbConf := config.InitializeConfigs()

	err := db.RunMigrations(ctx, dbConf)
	if err != nil {
		log.Fatalf("Error running migrations: %v", err)
		os.Exit(1)
	}

	dbPool, err := db.CreateConnection(ctx, dbConf)

	log.Println("Connecting to database")
	if err != nil {
		log.Fatalf("Unable to crate db connection: %v", err)
		os.Exit(1)
	}

	log.Println("Successfully connected to database")
	defer dbPool.Close()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)

	repository := repository.NewScrapperRepository(dbPool)
	service := service.NewScraperService(repository)

	orch := orchestrator.NewOrchestrator(service)

	go orch.RunOrchestrator(ctx)

	pb.RegisterScraperServer(s, &Server{
		ScrapperService: service,
	})
	log.Printf("Server listening on port %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
