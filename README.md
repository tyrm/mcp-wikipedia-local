# mcp-wikipedia-local

A local MCP (Model Context Protocol) server that serves Wikipedia articles from a compressed archive via SSE, with hybrid BM25+KNN search backed by Manticore Search and Ollama embeddings.

## Features

- MCP SSE server exposing `search` and `get_page` tools
- Hybrid search: BM25 full-text + KNN vector search via Manticore Search
- Embedding generation via Ollama
- In-memory LRU page cache
- Prometheus metrics endpoint
- OpenTelemetry tracing (OTLP/gRPC)
- Wikipedia XML archive scanning with checkpoint support

## Requirements

- Go 1.25+
- Manticore Search (MySQL-compatible endpoint on port 9306)
- Ollama (for embedding generation)
- Wikipedia XML dump (multistream bzip2 archive + index)

## Installation

```sh
go build -o mcp-wiki-local ./cmd/mcp-wiki-local
```

## Configuration

All flags can also be set via environment variables or a config file (YAML/TOML/JSON) specified with `--config-path`.

### Server flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server-host` | `0.0.0.0` | Listen host |
| `--server-port` | `8080` | Listen port |
| `--archive-path` | | Path to Wikipedia multistream bzip2 dump |
| `--archive-index-path` | | Path to the accompanying index file |
| `--manticore-dsn` | `root:@tcp(localhost:9306)/` | Manticore DSN |
| `--manticore-table` | `wiki_articles` | Table name |
| `--embed-url` | `http://localhost:11434` | Ollama base URL |
| `--embed-model` | `nomic-embed-text` | Embedding model |
| `--embed-dims` | `768` | Embedding dimensions |
| `--cache-max-size-mb` | `512` | Page cache size in MB |
| `--search-default-limit` | `10` | Default search result count |
| `--obs-prometheus-port` | `9090` | Prometheus metrics port |
| `--obs-otel-endpoint` | | OTLP gRPC endpoint (e.g. `localhost:4317`) |
| `--obs-service-name` | `mcp-wikipedia-local` | OTel service name |

### Scan flags

| Flag | Default | Description |
|------|---------|-------------|
| `--scan-workers` | `4` | Parallel indexing workers |
| `--scan-batch-size` | `100` | Articles per batch |
| `--scan-checkpoint-file` | | Resume file path |

## Usage

### Index Wikipedia articles

```sh
./mcp-wiki-local scan \
  --archive-path /data/wiki-multistream.xml.bz2 \
  --archive-index-path /data/wiki-multistream-index.txt.bz2
```

### Start the MCP server

```sh
./mcp-wiki-local server \
  --archive-path /data/wiki-multistream.xml.bz2 \
  --archive-index-path /data/wiki-multistream-index.txt.bz2
```

## API

### MCP Tools

**`search`** — search Wikipedia articles

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Search query |
| `limit` | number | no | Max results (default 10) |

**`get_page`** — retrieve a Wikipedia article

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `title` | string | yes | Article title |
| `mode` | string | no | `full`, `lead`, `outline`, or `section` (default `lead`) |
| `section` | string | no | Section name when `mode=section` |

### HTTP Endpoints

| Path | Description |
|------|-------------|
| `GET /sse` | MCP SSE connection endpoint |
| `POST /message?sessionId=<id>` | MCP message endpoint |
| `GET /health` | Returns `{"status":"ok"}` |
| `GET /metrics` (port 9090) | Prometheus metrics |

## Architecture

```
cmd/mcp-wiki-local/
  action/server/   — wires all components and starts the MCP server
  action/scan/     — indexes Wikipedia archive into Manticore

internal/
  archive/         — reads Wikipedia multistream bzip2 dumps
  cache/           — in-memory LRU page cache (ristretto)
  config/          — flag keys and default values
  embed/ollama/    — Ollama embedding client
  kv/              — key-value helpers
  logic/           — business logic (search, get_page)
  mcp/             — MCP SSE server wrapping logic
  obs/             — Prometheus metrics + OpenTelemetry tracing
  parser/          — wikitext parser
  search/manticore — Manticore Search client
  worker/          — parallel archive scanning worker
```

## Development

### Running Tests

```sh
go test -race ./...
```

### Build

```sh
go build ./...
go vet ./...
```
