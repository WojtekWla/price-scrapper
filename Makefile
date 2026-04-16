ifneq (,$(wildcard ./.env))
    include .env
    export
endif

run:
	@echo "Starting the scraper server..."
	go run .