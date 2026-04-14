package main

import (
	"context"
	"log"
	pb "price-scrapper/proto_gen"
	"price-scrapper/service"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type Server struct {
	pb.UnimplementedScraperServer
	ScrapperService service.Service
}

func (s *Server) RegisterProduct(ctx context.Context, in *pb.ScrapProductRequest) (*pb.ScrapProductReply, error) {

	//register product in db and time

	s.ScrapperService.RegisterProduct(service.Product{Name: in.Product, Frequency: in.Frequency})
	return &pb.ScrapProductReply{Message: "Product " + in.Product + " registered"}, nil
}

func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("Client Address: %s", p.Addr.String())
	}

	log.Printf("Incoming connection: %s", info.FullMethod)
	resp, err := handler(ctx, req)

	duration := time.Since(start)
	log.Printf("Reqiest handling duration: %v", duration)

	return resp, err
}
