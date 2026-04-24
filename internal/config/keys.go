package config

// KeyNames is a struct that contains the names of keys.
type KeyNames struct {
	LogLevel        string
	ConfigPath      string
	SoftwareVersion string

	// Server
	ServerHost string
	ServerPort string
	AuthToken  string

	// Archive
	ArchivePath      string
	ArchiveIndexPath string

	// Manticore
	ManticoreDSN       string
	ManticoreTable     string
	ManticoreBatchSize string

	// Embedding
	EmbedProvider  string
	EmbedURL       string
	EmbedAPIKey    string
	EmbedModel     string
	EmbedDims      string
	EmbedBatchSize string

	// Cache
	CacheMaxSizeMB string

	// Search
	SearchBM25Weight   string
	SearchKNNWeight    string
	SearchRRFK         string
	SearchDefaultLimit string

	// Scanner
	ScanWorkers        string
	ScanBatchSize      string
	ScanCheckpointFile string

	// Wikitext
	WikitextStripInfoboxes string
	WikitextStripTables    string

	// Observability
	ObsPrometheusPort string
	ObsOTELEndpoint   string
	ObsServiceName    string
}

// Keys contains the names of config keys.
var Keys = KeyNames{
	ConfigPath:      "config-path", // CLI only
	LogLevel:        "log-level",
	SoftwareVersion: "software-version", // Set at build

	// Server
	ServerHost: "server-host",
	ServerPort: "server-port",
	AuthToken:  "auth-token",

	// Archive
	ArchivePath:      "archive-path",
	ArchiveIndexPath: "archive-index-path",

	// Manticore
	ManticoreDSN:       "manticore-dsn",
	ManticoreTable:     "manticore-table",
	ManticoreBatchSize: "manticore-batch-size",

	// Embedding
	EmbedProvider:  "embed-provider",
	EmbedURL:       "embed-url",
	EmbedAPIKey:    "embed-api-key",
	EmbedModel:     "embed-model",
	EmbedDims:      "embed-dims",
	EmbedBatchSize: "embed-batch-size",

	// Cache
	CacheMaxSizeMB: "cache-max-size-mb",

	// Search
	SearchBM25Weight:   "search-bm25-weight",
	SearchKNNWeight:    "search-knn-weight",
	SearchRRFK:         "search-rrf-k",
	SearchDefaultLimit: "search-default-limit",

	// Scanner
	ScanWorkers:        "scan-workers",
	ScanBatchSize:      "scan-batch-size",
	ScanCheckpointFile: "scan-checkpoint-file",

	// Wikitext
	WikitextStripInfoboxes: "wikitext-strip-infoboxes",
	WikitextStripTables:    "wikitext-strip-tables",

	// Observability
	ObsPrometheusPort: "obs-prometheus-port",
	ObsOTELEndpoint:   "obs-otel-endpoint",
	ObsServiceName:    "obs-service-name",
}
