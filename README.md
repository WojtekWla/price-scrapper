# price-scrapper

A gRPC service that tracks product prices across the web on a schedule. Register a product name and a frequency, and the service searches for it, scrapes the top results, extracts structured price data with Gemini, stores the history in Postgres, and (optionally) posts the findings to a Discord webhook.

## How it works

1. **Register** a product via the gRPC API (`RegisterProduct`) with a name and a scraping frequency.
2. The **orchestrator** polls the database for jobs that are due to run and dispatches up to 5 concurrently.
3. For each job, the **scraper** (headless Chromium via [go-rod](https://github.com/go-rod/rod), with stealth patches) searches DuckDuckGo and crawls the top result pages, collecting Schema.org JSON-LD and visible page text.
4. The scraped HTML/text is sent to **Gemini** (`llm/gemini.go`), which extracts structured product name/price/link data, rate-limited to a configurable requests-per-minute budget.
5. Results are saved to Postgres as product history, the job's next run time is recomputed from its frequency, and a **Discord** notification is sent if a webhook URL is configured.

## Architecture

```
main.go / server.go   gRPC server entrypoint, wires everything together
config/               env-based configuration loading
db/                   Postgres connection + golang-migrate migrations
models/               shared domain types (Product, Job, ScrapedProduct)
repository/           Postgres queries
service/              business logic sitting on top of the repository
orchestrator/         polls for due jobs and runs them concurrently
orchestrator/scraper/ headless-browser search + page scraping
llm/                  Gemini client for structured extraction
discord/              Discord webhook notifier
proto_gen/            generated code from scraper.proto
client/               small CLI client for calling the gRPC API
```

## Requirements

- Go 1.25+
- PostgreSQL
- A Gemini API key
- Chromium (or set `CHROME_BIN` to a Chrome/Chromium binary) for local (non-Docker) scraping

## Configuration

The service is configured entirely through environment variables (see `config/config.go`). Create a `.env` file in the project root — the `Makefile` and Docker Compose both load it automatically.

| Variable            | Required | Description                                                        |
|---------------------|----------|----------------------------------------------------------------------|
| `DB_USER`           | yes      | Postgres user                                                        |
| `DB_PASSWORD`       | yes      | Postgres password                                                    |
| `DB_NAME`           | yes      | Postgres database name                                               |
| `DB_ADDRESS`        | yes      | Postgres host                                                        |
| `DB_PORT`           | yes      | Postgres port                                                        |
| `MIGRATIONS`        | yes      | Migration source URL, e.g. `file://db/migrations`                    |
| `GEMINI_API_KEY`    | yes      | Google Gemini API key                                                |
| `GEMINI_RPM`        | no       | Gemini requests-per-minute budget (default `15`)                     |
| `DISCORD_WEBHOOK_URL` | no     | Discord webhook URL for scrape result notifications                  |

Example `.env`:

```env
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=price_scrapper
DB_ADDRESS=localhost
DB_PORT=5432
MIGRATIONS=file://db/migrations
GEMINI_API_KEY=your-gemini-api-key
GEMINI_RPM=15
DISCORD_WEBHOOK_URL=
```

Database migrations run automatically on startup.

## Running locally

```bash
make run
```

This runs `go run .`, which starts the gRPC server on `:50051` and kicks off the orchestrator in the background.

## Running with Docker

```bash
docker-compose up
```

This starts Postgres and the app container together, running migrations against the containerized database.

## Using the client

A small CLI client is included for registering products against a running server:

```bash
go run ./client -product "Play Station 5" -frequency daily -host
```

- `-host`: connect to `localhost:50051` instead of the built-in test host
- `-product`: product name to search for and track
- `-frequency`: one of the keys in `service/frequency_handler.go` (`every minute`, `every 5 minutes`, `hourly`, `daily`)

## gRPC API

Defined in `scraper.proto`:

- `RegisterProduct(product, frequency)` — start tracking a product
- `ListProducts()` — list all registered scrape jobs and their next run time
- `GetProductHistory(product_name)` — fetch recorded price history for a product
- `DeleteProduct(product_name)` — stop tracking a product
