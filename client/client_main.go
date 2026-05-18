package main

import (
	"context"
	"log"
	pb "price-scrapper/proto_gen"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("38.49.212.226:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}
	defer conn.Close()

	c := pb.NewScraperClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.RegisterProduct(ctx, &pb.ScrapProductRequest{Product: "New Balance 530", Frequency: "daily"})
	if err != nil {
		log.Fatalf("Response error: %v", err)
	}

	log.Printf("Registered product: %s", r.GetMessage())
}
