package config

// Values contains the type of each value.
type Values struct {
	LogLevel        string
	ConfigPath      string
	SoftwareVersion string

	// Server
	ServerHost string
	ServerPort int
	AuthToken  string

	// Archive
	ArchivePath      string
	ArchiveIndexPath string

	// Manticore
	ManticoreDSN       string
	ManticoreTable     string
	ManticoreBatchSize int

	// Embedding
	EmbedProvider  string
	EmbedURL       string
	EmbedModel     string
	EmbedDims      int
	EmbedBatchSize int

	// Cache
	CacheMaxSizeMB int64

	// Search
	SearchBM25Weight   float64
	SearchKNNWeight    float64
	SearchRRFK         int
	SearchDefaultLimit int

	// Scanner
	ScanWorkers        int
	ScanBatchSize      int
	ScanCheckpointFile string

	// Wikitext
	WikitextStripInfoboxes bool
	WikitextStripTables    bool

	// Observability
	ObsPrometheusPort int
	ObsOTELEndpoint   string
	ObsServiceName    string
}

// Defaults contains the default values.
var Defaults = Values{
	ConfigPath:      "",
	LogLevel:        "info",
	SoftwareVersion: "dev",

	// Server
	ServerHost: "0.0.0.0",
	ServerPort: 8080,
	AuthToken:  "",

	// Archive
	ArchivePath:      "",
	ArchiveIndexPath: "",

	// Manticore
	ManticoreDSN:       "root:@tcp(localhost:9306)/",
	ManticoreTable:     "wiki_articles",
	ManticoreBatchSize: 1000,

	// Embedding
	EmbedProvider:  "ollama",
	EmbedURL:       "http://localhost:11434",
	EmbedModel:     "nomic-embed-text",
	EmbedDims:      768,
	EmbedBatchSize: 32,

	// Cache
	CacheMaxSizeMB: 512,

	// Search
	SearchBM25Weight:   0.5,
	SearchKNNWeight:    0.5,
	SearchRRFK:         60,
	SearchDefaultLimit: 10,

	// Scanner
	ScanWorkers:        4,
	ScanBatchSize:      100,
	ScanCheckpointFile: "",

	// Wikitext
	WikitextStripInfoboxes: true,
	WikitextStripTables:    false,

	// Observability
	ObsPrometheusPort: 9090,
	ObsOTELEndpoint:   "",
	ObsServiceName:    "mcp-wikipedia-local",
}
