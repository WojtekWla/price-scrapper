package main

import (
	"log"
	"net"
	pb "price-scrapper/proto_gen"
	"price-scrapper/service"

	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting server")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)
	pb.RegisterScraperServer(s, &Server{
		ScrapperService: service.NewScraperService(),
	})
	log.Printf("Server listening on port %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
