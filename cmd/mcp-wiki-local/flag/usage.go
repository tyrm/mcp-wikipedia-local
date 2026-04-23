package flag

import "github.com/tyrm/mcp-wikipedia-local/internal/config"

var usage = config.KeyNames{
	// Global
	ConfigPath:      "path to config file",
	LogLevel:        "log level (debug, info, warn, error)",
	SoftwareVersion: "software version (set at build time)",

	// Server
	ServerHost: "host address to listen on",
	ServerPort: "port to listen on",
	AuthToken:  "static bearer token for authentication (empty = disabled)",

	// Archive
	ArchivePath:      "path to the multistream bz2 Wikipedia dump file",
	ArchiveIndexPath: "path to the companion multistream index file",

	// Manticore
	ManticoreDSN:       "Manticore Search DSN (MySQL protocol)",
	ManticoreTable:     "Manticore table name for Wikipedia articles",
	ManticoreBatchSize: "number of articles per Manticore bulk insert",

	// Embedding
	EmbedProvider:  "embedding provider (ollama, openai, custom)",
	EmbedURL:       "base URL of the embedding service",
	EmbedModel:     "embedding model name",
	EmbedDims:      "embedding vector dimensions",
	EmbedBatchSize: "number of texts per embedding API call",

	// Cache
	CacheMaxSizeMB: "maximum in-process page cache size in megabytes",

	// Search
	SearchBM25Weight:   "RRF weight for BM25 results (0.0–1.0)",
	SearchKNNWeight:    "RRF weight for KNN results (0.0–1.0)",
	SearchRRFK:         "RRF constant k (higher = less rank-position sensitivity)",
	SearchDefaultLimit: "default number of search results to return",

	// Scanner
	ScanWorkers:        "number of parallel archive stream workers",
	ScanBatchSize:      "number of articles per scan batch",
	ScanCheckpointFile: "path to scan checkpoint file for resume support",

	// Wikitext
	WikitextStripInfoboxes: "strip infobox templates from wikitext output",
	WikitextStripTables:    "strip wikitable markup from wikitext output",

	// Observability
	ObsPrometheusPort: "port to serve Prometheus metrics on",
	ObsOTELEndpoint:   "OTLP gRPC endpoint for OpenTelemetry traces",
	ObsServiceName:    "service name reported in traces and metrics",
}
