package main

import (
	"context"
	"flag"
	"log"
	pb "price-scrapper/proto_gen"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	LOCALHOST = "localhost:50051"
	TEST_HOST = "38.49.212.226:50051"
)

func main() {
	host := flag.Bool("host", false, "Use localhost instead of test host")
	product := flag.String("product", "Play Station 5", "Product name")
	frequency := flag.String("frequency", "daily", "Scraping frequency")

	flag.Parse()

	var conn *grpc.ClientConn
	var err error

	if !*host {
		log.Print("Connecting to localhost")
		conn, err = grpc.NewClient(LOCALHOST, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		log.Print("Connecting to test server")
		conn, err = grpc.NewClient(TEST_HOST, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}
	defer conn.Close()

	c := pb.NewScraperClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.RegisterProduct(ctx, &pb.ScrapProductRequest{Product: *product, Frequency: *frequency})
	if err != nil {
		log.Fatalf("Response error: %v", err)
	}

	log.Printf("Registered product: %s", r.GetMessage())
}
