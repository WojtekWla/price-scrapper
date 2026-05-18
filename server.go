package main

import (
	"context"
	"errors"
	"log"
	"price-scrapper/models"
	pb "price-scrapper/proto_gen"
	"price-scrapper/service"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedScraperServer
	ScrapperService service.Service
}

func (s *Server) RegisterProduct(ctx context.Context, in *pb.ScrapProductRequest) (*pb.ScrapProductReply, error) {
	err := s.ScrapperService.RegisterProduct(ctx, models.Product{Name: in.Product, Frequency: in.Frequency})
	if err != nil {
		if errors.Is(err, service.ErrorInvalidFrequencyValue) {
			return nil, status.Errorf(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, service.ErrorProductAlreadyExists) {
			return nil, status.Errorf(codes.AlreadyExists, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &pb.ScrapProductReply{Message: "Product " + in.Product + " registered"}, nil
}

func (s *Server) GetProductHistory(ctx context.Context, in *pb.GetProductHistoryRequest) (*pb.GetProductHistoryReply, error) {
	if in.ProductName == "" {
		return nil, status.Error(codes.InvalidArgument, "product_name is required")
	}

	products, err := s.ScrapperService.GetProductHistory(ctx, in.ProductName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	entries := make([]*pb.PriceEntry, len(products))
	for i, p := range products {
		entries[i] = &pb.PriceEntry{
			Name:      p.Name,
			Price:     p.Price,
			Link:      p.Link,
			ScrapedAt: p.Time,
		}
	}

	return &pb.GetProductHistoryReply{Entries: entries}, nil
}

func (s *Server) ListProducts(ctx context.Context, in *pb.ListProductsRequest) (*pb.ListProductsReply, error) {
	jobs, err := s.ScrapperService.ListProducts(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	products := make([]*pb.ProductInfo, len(jobs))
	for i, j := range jobs {
		products[i] = &pb.ProductInfo{
			Id:          j.Id,
			ProductName: j.ProductName,
			Frequency:   j.Frequency,
			NextRun:     j.TimeToRun,
		}
	}

	return &pb.ListProductsReply{Products: products}, nil
}

func (s *Server) DeleteProduct(ctx context.Context, in *pb.DeleteProductRequest) (*pb.DeleteProductReply, error) {
	if in.ProductName == "" {
		return nil, status.Error(codes.InvalidArgument, "product_name is required")
	}

	err := s.ScrapperService.DeleteProduct(ctx, in.ProductName)
	if err != nil {
		if errors.Is(err, service.ErrorProductNotFound) {
			return nil, status.Errorf(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &pb.DeleteProductReply{Message: "Product " + in.ProductName + " deleted"}, nil
}

func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()

	if p, ok := peer.FromContext(ctx); ok {
		log.Printf("Client address: %s", p.Addr.String())
	}

	log.Printf("Incoming request: %s", info.FullMethod)
	resp, err := handler(ctx, req)
	log.Printf("Request handled in %v", time.Since(start))

	return resp, err
}
