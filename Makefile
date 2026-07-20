ifneq (,$(wildcard ./.env))
    include .env
    export
endif

run:
	@echo "Starting the scraper server..."
	go run .

gateway:
	@echo "Starting the HTTP gateway (web UI) on :8080..."
	go run ./gateway

proto:
	@echo "Regenerating protobuf code into ./proto_gen ..."
	protoc --go_out=. --go-grpc_out=. scraper.proto

.PHONY: run gateway proto