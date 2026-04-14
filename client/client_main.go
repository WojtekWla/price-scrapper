package main

import (
	"context"
	"log"
	pb "price-scrapper/proto_gen"
	"time"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("unable to connect %v", err)
	}

	defer conn.Close()
	c := pb.NewScraperClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.RegisterProduct(ctx, &pb.ScrapProductRequest{Product: "Play Station 5", Frequency: "daily"})
	if err != nil {
		log.Fatalf("response error: %v", err)
	}

	log.Printf("Registered product: %s", r.GetMessage())
}
