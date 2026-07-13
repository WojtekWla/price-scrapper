package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	pb "price-scrapper/proto_gen"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const requestTimeout = 20 * time.Second

type gateway struct {
	client pb.ScraperClient
}

func main() {
	grpcAddr := flag.String("grpc-addr", "localhost:50051", "address of the main gRPC Scraper server")
	httpAddr := flag.String("http-addr", ":8080", "address the HTTP gateway listens on")
	webDir := flag.String("web-dir", "./web", "directory with the static frontend files")
	flag.Parse()

	conn, err := grpc.NewClient(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("unable to create gRPC client: %v", err)
	}
	defer conn.Close()

	g := &gateway{client: pb.NewScraperClient(conn)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/products", g.listProducts)
	mux.HandleFunc("POST /api/products", g.registerProduct)
	mux.HandleFunc("DELETE /api/products/{name}", g.deleteProduct)
	mux.HandleFunc("GET /api/history", g.getHistory)
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))

	log.Printf("HTTP gateway listening on %s, proxying to gRPC at %s", *httpAddr, *grpcAddr)
	if err := http.ListenAndServe(*httpAddr, withCORS(mux)); err != nil {
		log.Fatalf("gateway server error: %v", err)
	}
}

func (g *gateway) registerProduct(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Product   string   `json:"product"`
		Frequency string   `json:"frequency"`
		Sites     []string `json:"sites"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	reply, err := g.client.RegisterProduct(ctx, &pb.ScrapProductRequest{
		Product:   body.Product,
		Frequency: body.Frequency,
		Sites:     body.Sites,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": reply.GetMessage()})
}

func (g *gateway) listProducts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	reply, err := g.client.ListProducts(ctx, &pb.ListProductsRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	products := make([]map[string]any, 0, len(reply.GetProducts()))
	for _, p := range reply.GetProducts() {
		products = append(products, map[string]any{
			"id":           p.GetId(),
			"product_name": p.GetProductName(),
			"frequency":    p.GetFrequency(),
			"next_run":     p.GetNextRun(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": products})
}

func (g *gateway) deleteProduct(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	reply, err := g.client.DeleteProduct(ctx, &pb.DeleteProductRequest{ProductName: name})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": reply.GetMessage()})
}

func (g *gateway) getHistory(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("product")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter 'product' is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	reply, err := g.client.GetProductHistory(ctx, &pb.GetProductHistoryRequest{ProductName: name})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	entries := make([]map[string]any, 0, len(reply.GetEntries()))
	for _, e := range reply.GetEntries() {
		entries = append(entries, map[string]any{
			"name":       e.GetName(),
			"price":      e.GetPrice(),
			"link":       e.GetLink(),
			"scraped_at": e.GetScrapedAt(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// writeGRPCError maps a gRPC status code onto a matching HTTP status code so the
// frontend can react (e.g. 409 for an already-tracked product).
func writeGRPCError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	httpCode := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
	case codes.AlreadyExists:
		httpCode = http.StatusConflict
	case codes.NotFound:
		httpCode = http.StatusNotFound
	case codes.Unavailable:
		httpCode = http.StatusServiceUnavailable
	}
	writeJSON(w, httpCode, map[string]string{"error": st.Message()})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
