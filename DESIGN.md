# mcp-wikipedia-local — Design Document

**Go-based SSE MCP server for offline Wikipedia browsing via multistream BZ2 archive**

---

## Table of Contents

1. [Overview](#overview)
2. [Goals & Non-Goals](#goals--non-goals)
3. [Architecture Overview](#architecture-overview)
4. [Binary Commands](#binary-commands)
5. [Archive Reading — Multistream BZ2](#archive-reading--multistream-bz2)
6. [Manticore Search Schema](#manticore-search-schema)
7. [Indexing Pipeline](#indexing-pipeline)
8. [Embedding Strategy](#embedding-strategy)
9. [Search: BM25 + KNN + RRF](#search-bm25--knn--rrf)
10. [Wikitext → Markdown Conversion](#wikitext--markdown-conversion)
11. [Page Parsing & Section Model](#page-parsing--section-model)
12. [Pagination Modes](#pagination-modes)
13. [LRU Cache](#lru-cache)
14. [MCP Tools](#mcp-tools)
15. [SSE MCP Server](#sse-mcp-server)
16. [Configuration](#configuration)
17. [Redirect Handling](#redirect-handling)
18. [Disambiguation Pages](#disambiguation-pages)
19. [Title Resolution](#title-resolution)
20. [Observability](#observability)
21. [Authentication](#authentication)
22. [Project Structure](#project-structure)
23. [Docker Compose](#docker-compose)
24. [Key Dependencies](#key-dependencies)
25. [Future Work](#future-work)

---

## Overview

`mcp-wiki-local` is a single Go binary that exposes an offline Wikipedia archive as a Model Context Protocol (MCP) server over SSE. It provides two tools to LLM clients: `search` (semantic + full-text hybrid) and `get_page` (content retrieval with flexible pagination modes). All content is returned as clean Markdown.

The binary has two sub-commands: `server` (run the MCP server) and `scan` (scan the dump and populate Manticore Search). Manticore runs as a separate sidecar container.

**Dump target:** `enwiki-20260101-pages-articles-multistream.xml.bz2` plus its companion index `enwiki-20260101-pages-articles-multistream-index.txt.bz2`.

---

## Goals & Non-Goals

### Goals

- Offline-first: no external data dependencies at query time — the archive and Manticore index are fully local; the embedding service is local infrastructure (Ollama/LM Studio) not an external network dependency
- Fast article retrieval via direct byte-offset seeking into the bz2 archive
- High-quality hybrid search: BM25 full-text + KNN semantic, fused with RRF
- Flexible content slicing (full / lead / outline / section) so LLMs can control token spend
- Memory-bounded in-process LRU cache for parsed/converted pages
- Language-agnostic architecture with English as the first supported language

### Non-Goals

- Real-time Wikipedia sync or live API fallback
- Image or file asset retrieval
- Editing or write operations
- Multi-language in v1 (abstraction layer included, implementations deferred)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                    MCP Client (LLM)                  │
└────────────────────────┬────────────────────────────┘
                         │ SSE / HTTP
┌────────────────────────▼────────────────────────────┐
│                  mcp-wiki-local server                      │
│                                                      │
│  ┌──────────────┐   ┌───────────────────────────┐   │
│  │  MCP Server  │   │       Tool Handlers        │   │
│  │  (SSE/HTTP)  │──▶│  search_tool  get_page     │   │
│  └──────────────┘   └──────┬──────────┬──────────┘   │
│                            │          │              │
│  ┌─────────────────────┐   │  ┌───────▼──────────┐  │
│  │   Search Layer      │◀──┘  │   Page Cache     │  │
│  │  BM25 + KNN + RRF   │      │   (ristretto)    │  │
│  └──────────┬──────────┘      └───────┬──────────┘  │
│             │                         │             │
│  ┌──────────▼──────────┐  ┌───────────▼──────────┐  │
│  │  Manticore Client   │  │   Archive Reader      │  │
│  │  (MySQL protocol)   │  │   (multistream bz2)   │  │
│  └──────────┬──────────┘  └───────────┬──────────┘  │
└─────────────┼───────────────────────  ┼─────────────┘
              │ TCP:9306                │ seek / ReadAt
┌─────────────▼──────────┐  ┌──────────▼─────────────┐
│   Manticore Search      │  │  .xml.bz2 dump file    │
│   (sidecar container)   │  │  + index .txt.bz2      │
└─────────────────────────┘  └────────────────────────┘

┌─────────────────────────────────────────────────────┐
│                  mcp-wiki-local scan                        │
│                                                      │
│  Archive Reader → XML Parser → Lead Extractor        │
│       → Embedding Client → Manticore Bulk Insert     │
└─────────────────────────────────────────────────────┘
```

---

## Binary Commands

The binary is structured with [cobra](https://github.com/spf13/cobra):

```
mcp-wiki-local
├── server     Start the SSE MCP server
└── scan       Scan the archive and populate Manticore
```

### `mcp-wiki-local server`

```
mcp-wiki-local server [--config-path config.yaml] [--host 0.0.0.0] [--port 8080]
```

Starts the SSE MCP server. Requires Manticore to be reachable and the archive file to exist on disk. Does **not** require the scan to be complete — it will serve what Manticore has. Cache is empty at startup and warms on demand.

### `mcp-wiki-local scan`

```
mcp-wiki-local scan [--config-path config.yaml] [--resume] [--workers 8]
```

Scans the multistream archive from the beginning (or from checkpoint if `--resume`), extracts titles and lead paragraphs, generates embeddings via the configured external service, and bulk-inserts into Manticore. Progress is logged to stderr with a running count and estimated time remaining.

The `--resume` flag reads a checkpoint file (default `./mcp-wiki-local-scan.checkpoint`) containing the last successfully committed bz2 stream offset, enabling safe interruption and restart of a multi-hour indexing run.

---

## Archive Reading — Multistream BZ2

### How the Multistream Format Works

The Wikipedia multistream dump consists of two files:

- **`...-multistream.xml.bz2`** — the main data file. It is composed of many independent bz2 streams concatenated together. Each stream decompresses to an XML fragment containing approximately 100 articles. Crucially, each stream can be decompressed in isolation by seeking to its start offset.
- **`...-multistream-index.txt.bz2`** — a small companion index. When decompressed, each line is: `stream_byte_offset:page_id:page_title`

Multiple consecutive index lines sharing the same `stream_byte_offset` belong to the same bz2 stream.

### Offset Index Loading

At startup of both `server` and `scan`, the companion index is decompressed fully into memory as a sorted lookup table:

```go
type OffsetIndex struct {
    // sorted by StreamOffset for binary search
    entries []IndexEntry
    // title-keyed map for O(1) title lookup
    byTitle map[string]*IndexEntry
    // page_id keyed map
    byID    map[uint32]*IndexEntry
}

type IndexEntry struct {
    StreamOffset uint64  // byte offset of the bz2 stream in the main file
    PageID       uint32
    Title        string
    RedirectTo   string  // empty string = not a redirect
    // IsDisambig is intentionally absent: the flag is looked up from
    // Manticore's is_disambig column at page-read time rather than being
    // stored in the in-memory index, since it is only needed on get_page
    // calls and would add ~7MB of heap for rarely-accessed data.
}
```

The English Wikipedia index (~7M lines, one per article) is roughly 200 MB uncompressed but only ~30–40 MB in memory as a compact struct slice. Load time is approximately 10–15 seconds; this is acceptable at startup. The index is loaded once and held in memory for the lifetime of the process.

### Stream Reading

```go
type StreamReader struct {
    f             *os.File  // the main .xml.bz2 file, opened once at startup
    maxStreamSize int64     // from config: archive.max_stream_size_bytes
}

func (r *StreamReader) ReadArticle(entry *IndexEntry) (wikitext string, err error) {
    // io.NewSectionReader uses ReadAt internally, which is safe for concurrent
    // use on *os.File — no seek or mutex required.
    // r.maxStreamSize comes from archive.max_stream_size_bytes in config.
    sr := io.NewSectionReader(r.f, int64(entry.StreamOffset), r.maxStreamSize)
    bz := bzip2.NewReader(sr)
    // parse XML fragment, find <page> with matching PageID, extract <text>
}
```

`*os.File.ReadAt` is concurrency-safe on all supported platforms — multiple goroutines can call `ReadArticle` simultaneously on the same `StreamReader` without coordination. The indexer workers share a single `StreamReader` for the same reason; no per-worker file handles are needed.

### XML Parsing

Each decompressed stream is a partial XML document (no root element). The reader wraps it with synthetic `<mediawiki>` tags before passing to `encoding/xml`. Only `<page>`, `<title>`, `<id>`, `<ns>`, and `<text>` elements are extracted. Pages with `<ns>` != `0` (non-article namespaces: Talk, User, Wikipedia, File, etc.) are skipped.

---

## Manticore Search Schema

Manticore is accessed via its MySQL-compatible protocol using the standard `database/sql` + `go-sql-driver/mysql` driver.

### Table: `wiki_articles`

```sql
CREATE TABLE wiki_articles (
    id          BIGINT,           -- Wikipedia page_id (used as Manticore doc id)
    title       TEXT,             -- article title, indexed for full-text
    lead_text   TEXT,             -- lead paragraph plain text, indexed for full-text
    lang        STRING,           -- language code, e.g. "en"
    is_disambig   BOOL,           -- true for disambiguation pages
    lead_embedding FLOAT_VECTOR KNNL2('hnsw', 'M=16, ef=200') -- KNN index
                   DIMENSIONS=768  -- must match embedding model output
)
engine='columnar'
morphology='stem_en'
index_fields='title, lead_text'
stored_fields='title, lead_text, lang, is_disambig';
```

**Notes:**
- `id` is set to Wikipedia's `page_id` — globally unique, avoids a separate mapping table.
- `lead_embedding` dimensions must match the configured embedding model. The schema DDL is generated at `index` time from config, not hardcoded.
- `engine='columnar'` is preferred for large datasets with mixed attribute access patterns.
- `morphology='stem_en'` enables English stemming for BM25. This becomes a config-driven parameter for future language support.
- Article byte offsets are kept exclusively in the in-memory `OffsetIndex`; `stream_offset` is not stored in Manticore since the in-memory index is the authoritative source for bz2 seek positions.

---

## Indexing Pipeline

The `mcp-wiki-local scan` command runs the following pipeline:

```
Archive Streams
      │
      ▼
┌─────────────┐     N worker goroutines
│ Stream Queue │──▶ Parse XML → Extract pages (ns=0 only)
└─────────────┘         │
                        ▼
                  Lead Extraction
                  (wikitext → plain text lead, ~first 2 paragraphs)
                        │
                        ▼
                  Embedding Batcher
                  (accumulate until batch_size, then call embed API)
                        │
                        ▼
                  Manticore Bulk Insert
                  (INSERT INTO wiki_articles VALUES (...), (...), ...)
                        │
                        ▼
                  Checkpoint Write
                  (every checkpoint_interval batches: last committed stream offset → file)
```

### Lead Extraction for Indexing

During indexing we do **not** run the full wikitext→markdown converter. Instead, a lightweight plain-text extractor strips the most common wikitext markup from the lead section to produce clean text for BM25 indexing and embedding. This is much faster than the full converter and sufficient for search quality.

The extractor: removes `{{...}}` templates, strips `[[link|display]]` to `display`, removes `''`, `'''`, HTML tags, and truncates at the first `==` heading or after 500 words, whichever comes first.

### Concurrency Model

```
main goroutine: reads index entries, pushes stream offsets to streamCh

streamCh (buffered, size=workers*4)
    │
    ├── worker 0: ReadArticle → extractLead → push to embedCh
    ├── worker 1: ...
    └── worker N: ...

embedCh (buffered, size=embed_concurrency*4)
    │
    ├── embedWorker 0: accumulate batch → POST /embeddings → push to insertCh
    └── embedWorker M: ...

insertCh (buffered, size=16)
    │
    └── insertWorker: bulk INSERT into Manticore → write checkpoint
```

The embedding workers are intentionally fewer than parse workers since embedding API calls are the bottleneck (network + GPU).

### Estimated Indexing Time

English Wikipedia has ~7 million articles. With a local Ollama instance on an RTX 4090:
- Parsing + lead extraction: ~2–3 hours (CPU-bound, 8 workers)
- Embedding (nomic-embed-text, batch=32): ~3–5 hours (GPU-bound)
- Total wall clock with parallelism: ~5–8 hours

The checkpoint system makes interruption and resumption safe.

---

## Embedding Strategy

### External Embedding Client

The embedding client is defined by an interface, allowing any provider:

```go
type EmbedClient interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
}
```

Concrete implementations:

| Provider | `embedding.provider` | Endpoint |
|---|---|---|
| Ollama | `ollama` | `POST /api/embed` |
| OpenAI-compatible | `openai` | `POST /v1/embeddings` |
| Custom HTTP | `custom` | configurable `base_url` + `path` |

### Recommended Model

`nomic-embed-text` (768 dims) is the recommended default — widely available via Ollama, strong performance on factual/encyclopedic text, and reasonable index size (~7M × 768 × 4 bytes ≈ ~21 GB for the full English Wikipedia vector index).

If index size is a concern, `mxbai-embed-large` at 1024 dims offers better quality but larger footprint; `all-minilm` at 384 dims halves the storage at some quality cost.

### Query-Time Embedding

At search time, the query string is embedded using the same client and model. This is a single API call per `search` invocation. If the embedding service is unavailable, `search` degrades gracefully to BM25-only with a logged warning.

---

## Search: BM25 + KNN + RRF

### Two Queries, One Ranked List

For a query `q`:

**Step 1 — BM25 query:**

Before being passed to `MATCH(?)`, the user query is sanitised by stripping Manticore full-text query operators that would otherwise cause parse errors or unexpected behaviour: `@`, `|`, `!`, `^`, `~`, `(`, `)`, `"`, `/`, `\`. The sanitised string is then passed as a plain keyword query.

```sql
SELECT id, title, lead_text, WEIGHT() AS bm25_score
FROM wiki_articles
WHERE MATCH(?) AND lang = ?
LIMIT 100
OPTION ranker=bm25, field_weights=(title=3, lead_text=1)
```
Title matches are weighted 3× vs lead text.

**Step 2 — KNN query:**
```sql
SELECT id, title, lead_text
FROM wiki_articles
WHERE KNN(lead_embedding, 100, ?)
  AND lang = ?
LIMIT 100
```

**Step 3 — RRF fusion in Go:**

```go
// k=60 is standard RRF constant; dampens the effect of rank position
func RRF(bm25Results, knnResults []SearchResult, k int) []SearchResult {
    scores := make(map[uint32]float64)

    for rank, r := range bm25Results {
        scores[r.PageID] += 1.0 / float64(k + rank + 1)
    }
    for rank, r := range knnResults {
        scores[r.PageID] += 1.0 / float64(k + rank + 1)
    }

    // merge unique results, sort by RRF score descending
    return sortByScore(mergeUnique(bm25Results, knnResults), scores)
}
```

**Step 4 — Return top N** (default 10, configurable per call up to 50).

### BM25-Only Fallback

If the embedding service is unreachable at query time, the server falls back to BM25 only and the footer note in the Markdown output reflects this:

```markdown
*Search mode: BM25 only (embedding service unavailable). 10 results shown.*
```

---

## Wikitext → Markdown Conversion

Conversion happens at read time, on cache miss. The converter is a custom Go implementation covering the markup constructs most relevant to LLM consumption. It deliberately discards or simplifies constructs that are visually meaningful but semantically noisy.

### Conversion Rules

| Wikitext | Markdown Output |
|---|---|
| `== Heading ==` | `## Heading` |
| `=== Sub ===` | `### Sub` |
| `''italic''` | `*italic*` |
| `'''bold'''` | `**bold**` |
| `[[Page Title]]` | `Page Title` (link stripped, title kept) |
| `[[Page Title\|display]]` | `display` |
| `[https://example.com label]` | `[label](https://example.com)` |
| `{{Infobox ...}}` | structured key-value block or omitted (see below) |
| `{{reflist}}`, `{{cite ...}}` | omitted |
| `{| ... |}` (wikitable) | GFM table |
| `<ref>...</ref>` | omitted |
| `<math>...</math>` | `$...$` (LaTeX passthrough) |
| `[[File:...]]`, `[[Image:...]]` | `[Image: caption]` or omitted |
| `<gallery>` | omitted |
| `#REDIRECT` | `> Redirects to: [[Target]]` *(safety fallback — should be unreachable in normal operation since `get_page` follows redirects before invoking the converter)* |

### Infobox Handling

Infoboxes are the highest-value templates for LLMs. The converter parses `{{Infobox ...}}` into a fenced block:

```
> **[Infobox: Person]**
> Born: 1 January 1970
> Nationality: American
> Occupation: Physicist
```

Unknown or deeply nested templates outside of Infobox patterns are stripped entirely. A template allowlist (e.g. `{{convert}}`, `{{lang}}`, `{{IPA}}`) can be extended via config.

### Wikitable → GFM Table

Wikitables are converted to GitHub Flavored Markdown tables. Colspan/rowspan cells are flattened with repeated values. If a table is malformed beyond recovery, it is replaced with `[Table omitted]`.

### Implementation Notes

The converter is implemented as a single-pass state machine over the wikitext string. It does **not** use a full parse tree (which would require a heavy dependency) but handles nesting of `{{` / `}}` and `[[` / `]]` via a depth counter. This is sufficient for >95% of Wikipedia articles; edge cases are logged and the raw wikitext segment is dropped rather than emitted garbage.

**Dependency:** [`golang.org/x/net/html`](https://pkg.go.dev/golang.org/x/net/html) for stripping inline HTML tags. No external wikitext library dependency.

---

## Page Parsing & Section Model

After wikitext→markdown conversion, the result is parsed into a structured `ParsedPage`. This struct is what gets cached.

```go
type ParsedPage struct {
    PageID   uint32
    Title    string
    Lang     string
    Lead     string     // markdown content before the first H2
    Sections []Section
    // precomputed for outline mode
    Outline  []OutlineEntry
}

type Section struct {
    Level   int    // 2 = H2, 3 = H3, etc.
    Title   string
    Content string // markdown body of this section (heading line included)
    Bytes   int    // len(Content)
}

type OutlineEntry struct {
    Level int    `json:"level"`
    Title string `json:"title"`
    Bytes int    `json:"bytes"`
}
```

Sections are identified by scanning the converted markdown for lines matching `^#{2,6} .+`. The `Lead` field is everything before the first `## ` line. The `Sections` slice is split at H2 boundaries only — each H2's `Content` string includes all nested H3–H6 content up to the next H2. The `Outline` field contains entries for H2 and H3 headings only (not H4+); H3 `bytes` counts only that H3's content up to the next H3 or H2. The `section` pagination mode only accepts H2 titles as the `section` parameter.

---

## Pagination Modes

All `get_page` modes operate as views over the cached `ParsedPage`. No re-parsing occurs on subsequent calls for different modes of the same page.

### `mode: "full"`

Returns `Lead + all Section.Content` joined. This is the complete article as Markdown. For large articles this can be 50–200 KB of text; LLM callers should use this sparingly.

### `mode: "lead"`

Returns `ParsedPage.Lead` only. Typically 200–1000 words. Contains the introduction paragraph(s) and infobox. The recommended default for initial article retrieval.

### `mode: "outline"`

Returns `ParsedPage.Outline` as a JSON array embedded in a Markdown code block. The outline includes H2 and H3 headings only — deep enough for meaningful navigation without overwhelming the LLM with H4–H6 subsections.

```markdown
## Outline: Alan Turing

```json
[
  {"level": 2, "title": "Early life and education", "bytes": 3240},
  {"level": 3, "title": "Childhood and family", "bytes": 980},
  {"level": 3, "title": "School", "bytes": 1100},
  {"level": 2, "title": "University and work on computability", "bytes": 8820},
  {"level": 2, "title": "Cryptanalysis of the Enigma", "bytes": 12400},
  ...
]
```
```

The `bytes` field reflects the byte length of that heading's content block (for H3s, only up to the next H3 or H2). This lets the LLM estimate token cost before fetching a section. Note: the `section` mode only accepts H2 titles — H3s are included in the content of their parent H2.

This mode is the cheapest way to discover section names for a subsequent `section` call.

### `mode: "section"`, `section: "<H2 title>"`

Returns the `Section.Content` for the matching H2. Matching is case-insensitive and trims leading/trailing whitespace. If the section is not found, an error is returned listing available section titles.

Returns one H2 and all its nested H3+ content, up to (but not including) the next H2.

---

## LRU Cache

### Library: `github.com/dgraph-io/ristretto/v2`

Ristretto is chosen for its production-grade admission policy (TinyLFU), memory-bounded operation, and thread safety. It tracks memory cost in bytes, not item count.

```go
type PageCache struct {
    cache *ristretto.Cache[string, *ParsedPage]  // requires ristretto v2
}

func NewPageCache(maxSizeMB int) (*PageCache, error) {
    c, err := ristretto.NewCache(&ristretto.Config[string, *ParsedPage]{
        NumCounters: 200_000,        // 10x expected max items (~17k articles at 512MB/30KB avg)
        MaxCost:     int64(maxSizeMB) * 1024 * 1024,
        BufferItems: 64,
        Cost: func(p *ParsedPage) int64 {
            return int64(len(p.Lead) + totalSectionBytes(p))
        },
    })
    return &PageCache{cache: c}, err
}

func (c *PageCache) Key(lang, title string) string {
    return lang + ":" + title
}
```

Cache keys are `"lang:normalized_title"` where normalization lowercases and collapses spaces to underscores (matching Wikipedia's canonical form).

### Cache Sizing

A typical English Wikipedia article converted to Markdown is 15–60 KB. At 512 MB cache max with an average of 30 KB per article, the cache holds ~17,000 articles. This covers a working set comfortably for a single LLM session or small multi-user deployment. The `max_size_mb` config value should be tuned to available RAM.

---

## MCP Tools

### Tool: `search`

**Description:** Search Wikipedia articles by title and content using hybrid semantic + full-text search.

**Input schema:**
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Natural language search query"
    },
    "limit": {
      "type": "integer",
      "description": "Number of results to return (1–50, default 10)",
      "default": 10
    },
    "lang": {
      "type": "string",
      "description": "Language code (default: en)",
      "default": "en"
    }
  },
  "required": ["query"]
}
```

**Output:** Markdown-formatted result list.

```markdown
## Search Results for "quantum entanglement"

1. **Quantum entanglement** — Quantum entanglement is a phenomenon where two or more particles become correlated such that the quantum state of each cannot be described independently...
   `get_page(title="Quantum entanglement")`

2. **Bell's theorem** — Bell's theorem is a mathematical theorem that shows that quantum mechanics is incompatible with local hidden-variable theories...
   `get_page(title="Bell's theorem")`

...

*Search mode: hybrid (BM25 + KNN). 10 results shown.*
```

Each snippet is the first 150 characters of the lead paragraph (sourced from Manticore's `lead_text` field — no bz2 read required for search results).

### Tool: `get_page`

**Description:** Retrieve a Wikipedia article or section in Markdown.

**Input schema:**
```json
{
  "type": "object",
  "properties": {
    "title": {
      "type": "string",
      "description": "Wikipedia article title — exact match attempted first, with case/underscore normalisation and fuzzy fallback. See Title Resolution."
    },
    "mode": {
      "type": "string",
      "enum": ["full", "lead", "outline", "section"],
      "description": "Content slice mode",
      "default": "lead"
    },
    "section": {
      "type": "string",
      "description": "Required when mode=section. The H2 section title to retrieve."
    },
    "lang": {
      "type": "string",
      "default": "en"
    }
  },
  "required": ["title"]
}
```

**Output:** Markdown string. The first line is always `# Article Title` followed by a blank line, then the mode-specific content.

**Error cases (returned as Markdown error blocks):**

```markdown
> **Error:** Article "Quantom Entanglement" not found.
> Did you mean: Quantum entanglement, Quantum entanglement (film)?
```

```markdown
> **Error:** Section "Early Life" not found in "Alan Turing".
> Available sections: Early life and education, University and work on computability, ...
```

### Typical LLM Usage Pattern

```
1. search(query="quantum computing error correction")
   → Returns 10 titles + snippets

2. get_page(title="Quantum error correction", mode="lead")
   → Returns intro/infobox (~500 tokens). LLM decides if it needs more.

3. get_page(title="Quantum error correction", mode="outline")
   → Returns section list + byte counts. LLM picks a section.

4. get_page(title="Quantum error correction", mode="section", section="Stabilizer codes")
   → Returns just that section (~2000 tokens).
```

---

## SSE MCP Server

### Library: `github.com/mark3labs/mcp-go`

`mcp-go` provides SSE transport and MCP protocol handling out of the box. The server is configured as:

```go
s := server.NewMCPServer(
    "mcp-wiki-local",
    "0.1.0",
    server.WithToolCapabilities(),
)

s.AddTool(searchTool, searchHandler)
s.AddTool(getPageTool, getPageHandler)

sseServer := server.NewSSEServer(s,
    server.WithBaseURL(fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)),
)
sseServer.Start(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
```

### Endpoints

| Path | Description |
|---|---|
| `GET /sse` | SSE stream — MCP client connects here |
| `POST /message` | MCP message endpoint |
| `GET /health` | Returns `{"status":"ok","index_ready":true}` |

### Error Handling

All tool handlers return structured MCP errors (not panics) for: article not found, Manticore unavailable, bz2 seek failure, embedding service timeout. The server never crashes on per-request errors.

---

## Configuration

Config is loaded via [Viper](https://github.com/spf13/viper) supporting a YAML file, environment variable overrides (`MCP_WIKI_LOCAL_` prefix), and CLI flags.

```yaml
# config.yaml

server:
  host: "0.0.0.0"
  port: 8080

archive:
  path: "/data/enwiki-20260101-pages-articles-multistream.xml.bz2"
  index_path: "/data/enwiki-20260101-pages-articles-multistream-index.txt.bz2"
  default_lang: "en"
  # Maximum compressed size of a single bz2 stream read into memory.
  # Wikipedia streams are typically 200–800 KB compressed. 10 MB is a
  # conservative upper bound that prevents runaway reads on corrupt data.
  max_stream_size_bytes: 10485760   # 10 MB

manticore:
  host: "manticore"
  port: 9306          # MySQL protocol
  table: "wiki_articles"
  dial_timeout: "5s"
  query_timeout: "10s"

embedding:
  provider: "ollama"            # ollama | openai | custom
  base_url: "http://ollama:11434"
  model: "nomic-embed-text"
  dimensions: 768
  batch_size: 32
  concurrency: 4               # parallel embed API calls during indexing
  timeout: "30s"
  # On search, if embed fails, fall back to BM25 only:
  fallback_to_bm25: true

cache:
  max_size_mb: 512
  num_counters: 10_000_000

search:
  default_limit: 10
  max_limit: 50
  rrf_k: 60                    # RRF constant
  bm25_candidate_limit: 100    # rows fetched from Manticore before RRF
  knn_candidate_limit: 100

scanner:
  workers: 8                   # parallel bz2 stream readers
  batch_size: 100              # articles per Manticore INSERT
  checkpoint_file: "./mcp-wiki-local-scan.checkpoint"
  checkpoint_interval: 1000    # write checkpoint every N batches committed to Manticore
                                # (1000 batches × 100 articles/batch = ~100k articles between checkpoints)

wikitext:
  # Templates to render as key-value blocks rather than strip
  infobox_prefix: "Infobox"
  allowed_templates:
    - "convert"
    - "lang"
    - "IPA"
    - "math"

observability:
  prometheus:
    enabled: true
    host: "0.0.0.0"
    port: 9090
    path: "/metrics"
  tracing:
    enabled: true
    exporter: "otlp"          # otlp | stdout | none
    otlp_endpoint: "localhost:4317"
    service_name: "mcp-wiki-local"
    sample_rate: 1.0          # 1.0 = trace everything

auth:
  enabled: false
  # bearer_token: "changeme"   # uncomment to enable static token auth
```

### Environment Variable Overrides

```bash
MCP_WIKI_LOCAL_EMBEDDING_BASE_URL=http://localhost:11434
MCP_WIKI_LOCAL_MANTICORE_HOST=localhost
MCP_WIKI_LOCAL_CACHE_MAX_SIZE_MB=1024
```

---

## Project Structure

```
mcp-wikipedia-local/
├── cmd/
│   └── mcp-wiki-local/
│       ├── action/
│       │   ├── action.go            # Action type: func(context.Context, []string) error
│       │   ├── scan/
│       │   │   └── scan.go          # scan action (archive indexing lifecycle)
│       │   └── server/
│       │       └── server.go        # server Start action (signal handling, lifecycle)
│       ├── flag/
│       │   ├── global.go            # --config-path persistent flag
│       │   ├── server.go            # server subcommand flags
│       │   └── usage.go             # flag usage strings
│       └── main.go                  # cobra root + subcommands (server, scan), zap init
│
├── internal/
│   ├── archive/
│   │   ├── archive.go               # Archive struct + New()
│   │   ├── config.go                # Archive config struct
│   │   ├── offset_index.go          # companion index loader & lookup        [planned]
│   │   ├── stream_reader.go         # multistream bz2 seek + ReadAt          [planned]
│   │   └── xml_parser.go            # XML → wikitext extraction              [planned]
│   │
│   ├── config/
│   │   ├── config.go                # Init(), ReadConfigFile() via viper
│   │   ├── keys.go                  # KeyNames struct + Keys (log-level, config-path, software-version)
│   │   └── values.go                # Values struct + Defaults
│   │
│   ├── cache/                       # [planned]
│   │   └── page_cache.go            # ristretto-backed LRU with cost function
│   │
│   ├── logic/
│   │   ├── config.go                # Logic config struct
│   │   └── logic.go                 # Logic struct + New() (central business logic)
│   │
│   ├── search/
│   │   ├── manticore/
│   │   │   ├── client.go            # Manticore Client struct + New()
│   │   │   └── config.go            # Manticore config struct
│   │   ├── search.go                # Search interface
│   │   ├── embed.go                 # EmbedClient interface + ollama/openai impls  [planned]
│   │   └── rrf.go                   # Reciprocal Rank Fusion                        [planned]
│   │
│   ├── wikitext/                    # [planned]
│   │   ├── converter.go             # wikitext → markdown state machine
│   │   ├── templates.go             # template handling (infobox, allowed list)
│   │   └── tables.go                # wikitable → GFM table
│   │
│   ├── page/                        # [planned]
│   │   ├── model.go                 # ParsedPage, Section, OutlineEntry types
│   │   ├── parser.go                # markdown → ParsedPage section splitter
│   │   └── renderer.go              # mode → markdown output (full/lead/outline/section)
│   │

│   ├── mcp/                         # [planned]
│   │   ├── server.go                # MCP server setup, SSE wiring
│   │   ├── search_tool.go           # search tool definition + handler
│   │   └── get_page_tool.go         # get_page tool definition + handler
│   │
│   └── observability/               # [planned]
│       ├── prometheus.go            # metric definitions + registration
│       └── tracing.go               # OTel SDK init, span helpers, OTLP exporter
│
├── docker/                          # [planned]
│   ├── docker-compose.yml
│   └── manticore.conf
│
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

> Files and packages marked `[planned]` do not yet exist in the repository but are specified by this design document. Existing files reflect the current scaffold state.

```

---

## Docker Compose

```yaml
# docker/docker-compose.yml
version: "3.9"

services:
  manticore:
    image: manticoresearch/manticore:6.3.6
    container_name: mcp-wiki-local-manticore
    restart: unless-stopped
    ports:
      - "9306:9306"   # MySQL protocol (internal use)
      - "9308:9308"   # HTTP API (optional, for debugging)
    volumes:
      - manticore-data:/var/lib/manticore
      - ./manticore.conf:/etc/manticoresearch/manticore.conf:ro
    ulimits:
      nofile:
        soft: 65536
        hard: 65536

  mcp-wiki-local:
    build: ..
    container_name: mcp-wiki-local
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "9090:9090"   # Prometheus metrics
    volumes:
      - /data/wikipedia:/data:ro    # mount the dump files read-only
      - ./config.yaml:/etc/mcp-wikipedia-local/config.yaml:ro
    environment:
      MCP_WIKI_LOCAL_MANTICORE_HOST: manticore
    depends_on:
      - manticore
    command: ["mcp-wiki-local", "server", "--config-path", "/etc/mcp-wikipedia-local/config.yaml"]

volumes:
  manticore-data:
```

```ini
# docker/manticore.conf
searchd {
    listen = 9306:mysql41
    listen = 9308:http
    log = /var/log/manticore/searchd.log
    pid_file = /var/run/manticore/searchd.pid
    data_dir = /var/lib/manticore
}
```

### Typical Workflow

```bash
# 1. Start Manticore
docker compose up -d manticore

# 2. Run the scan to index the archive (runs outside compose, reads from host data dir)
mcp-wiki-local scan --config-path config.yaml

# 3. Start the MCP server
docker compose up -d mcp-wiki-local

# 4. Connect your MCP client to http://localhost:8080/sse
```

---

## Redirect Handling

Wikipedia redirect pages have wikitext of the form `#REDIRECT [[Target Title]]`. During indexing the indexer detects this pattern and stores a lightweight redirect record in the in-memory offset index rather than inserting the page into Manticore:

Redirect articles are **not** inserted into Manticore (they have no meaningful lead text to search or embed).

When `get_page` is called with a redirect title, it resolves the target entry, fetches and parses the target article, and prepends a notice to the output:

```markdown
> *Redirected from: USA → United States*

# United States
...
```

One level of redirect is followed. Circular or chained redirects are detected by comparing the target title against the original request and the target's own `RedirectTo` field. If either matches, a hard error is returned:

```markdown
> **Error:** Circular redirect detected: "Example" → "Example".
```

---

## Disambiguation Pages

Articles containing `{{Disambiguation}}` or `{{disambig}}` (case-insensitive) in their wikitext are flagged during indexing via the `is_disambig BOOL` column in Manticore. The flag is **not** stored in `IndexEntry` — it is looked up from Manticore as part of the page-read path, keeping the in-memory index lean.

**Indexing:** Disambiguation pages are indexed normally — title + lead inserted into Manticore, embedded and searchable — so that a `search` for "Mercury" can surface the disambiguation page alongside specific articles.

**`get_page` on a disambiguation page:** On cache miss, the page-read path queries Manticore for `is_disambig` alongside the article fetch. If true, instead of running the standard pagination modes, the handler returns a structured listing of options extracted from the wikitext link list:

```markdown
# Mercury (disambiguation)

> *This is a disambiguation page. Select a more specific article:*

- **Mercury** — chemical element with symbol Hg
- **Mercury (planet)** — the innermost planet of the Solar System
- **Mercury (mythology)** — Roman god of commerce
- **Freddie Mercury** — vocalist of Queen
- **Mercury Records** — American record label

*Use `get_page(title="...")` with one of the titles above.*
```

Options are extracted by scanning the wikitext list items (`* [[Link|display]] — description text`). For each item the extractor captures:
- The link target (canonical article title)
- The display name if different from the target
- The inline description: any text after a ` — `, ` - `, or ` – ` separator on the same list item line

`File:`, `Category:`, and `Template:` namespace links are skipped. If no description is found for a link, only the title is emitted.

---

## Title Resolution

`get_page` resolves titles through the following ordered steps, stopping at the first match:

1. **Exact match** — look up the title as-is in the in-memory `byTitle` map. O(1).
2. **Normalised match** — lowercase + replace spaces with underscores (and vice versa). Handles `Alan_Turing` vs `Alan Turing` and common capitalisation variants.
3. **Fuzzy match via Manticore** — if steps 1–2 fail, issue a Manticore title-field query:
   ```sql
   SELECT id, title FROM wiki_articles
   WHERE MATCH('@title ?')
   ORDER BY WEIGHT() DESC
   LIMIT 5
   ```
   The results are returned as suggestions in a hard error:
   ```markdown
   > **Error:** Article "Quantom Entanglement" not found.
   > Did you mean: Quantum entanglement, Quantum field theory, Quantum mechanics?
   ```

If Manticore is unreachable during step 3, the error is returned without suggestions rather than blocking.

---

## Observability

The `server` command exposes both Prometheus metrics and OpenTelemetry traces.

### Prometheus

Metrics are served on a separate port (default `9090`) at `/metrics`:

| Metric | Type | Description |
|---|---|---|
| `mcp_wiki_local_search_requests_total` | Counter | Search tool invocations, labelled `status={ok,error}` |
| `mcp_wiki_local_get_page_requests_total` | Counter | get_page invocations, labelled `mode`, `status` |
| `mcp_wiki_local_search_duration_seconds` | Histogram | End-to-end search latency |
| `mcp_wiki_local_get_page_duration_seconds` | Histogram | End-to-end get_page latency, labelled `mode` |
| `mcp_wiki_local_cache_hits_total` | Counter | Ristretto cache hits |
| `mcp_wiki_local_cache_misses_total` | Counter | Ristretto cache misses |
| `mcp_wiki_local_bz2_reads_total` | Counter | Archive seek+decompress operations |
| `mcp_wiki_local_bz2_read_duration_seconds` | Histogram | Latency of bz2 stream reads |
| `mcp_wiki_local_embed_requests_total` | Counter | Embedding API calls at query time, labelled `status` |
| `mcp_wiki_local_manticore_queries_total` | Counter | Manticore queries, labelled `type={bm25,knn}`, `status` |

### OpenTelemetry

OTel tracing uses the standard Go SDK (`go.opentelemetry.io/otel`). Each tool invocation creates a root span; child spans cover significant sub-operations:

```
search("quantum entanglement")
├── embed_query          [embedding API call]
├── manticore_bm25       [BM25 SQL query]
├── manticore_knn        [KNN SQL query]
└── rrf_fusion           [in-process fusion]

get_page("Alan Turing", mode="section")
├── title_resolve        [offset index lookup + optional Manticore fallback]
├── cache_lookup         [ristretto get]
├── bz2_read             [seek + decompress, on cache miss]
├── wikitext_convert     [wikitext → markdown, on cache miss]
└── section_render       [extract requested section]
```

The OTLP exporter endpoint is configurable; it defaults to `localhost:4317` (gRPC). Tracing can be disabled by setting `observability.tracing.enabled: false`.

*See the [Configuration](#configuration) section for the full `observability:` YAML block.*

---

## Authentication

Authentication is intentionally omitted in v1 — the server is designed for trusted local or internal network deployment. A placeholder is present in the canonical config (see [Configuration](#configuration)) for future use. When `auth.enabled: true` and `bearer_token` is set, the server rejects any SSE connection or `/message` POST that does not present `Authorization: Bearer <token>`. This is a minimal guard against accidental exposure; it is not a substitute for network-level access controls.

---

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI sub-commands |
| `go.uber.org/zap` | Structured logging |
| `github.com/spf13/viper` | Config loading |
| `github.com/mark3labs/mcp-go` | MCP protocol + SSE server |
| `github.com/go-sql-driver/mysql` | Manticore MySQL protocol client |
| `github.com/dgraph-io/ristretto/v2` | In-process LRU/LFU cache (generics API) |
| `golang.org/x/net/html` | HTML tag stripping in wikitext converter |
| `compress/bzip2` | stdlib — bz2 decompression |
| `encoding/xml` | stdlib — XML stream parsing |
| `github.com/prometheus/client_golang` | Prometheus metrics exposition |
| `go.opentelemetry.io/otel` | OpenTelemetry tracing SDK |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | OTLP gRPC trace exporter |

---

## Future Work

**Multi-language.** The `lang` field is in the schema and config from day one. Adding a new language requires: a new dump + companion index, a separate Manticore table (or a filter on `lang`), and a language-specific morphology config for Manticore. The `EmbedClient` is language-agnostic.

**Streaming `get_page` for large articles.** The `full` mode can return very large responses. MCP SSE allows streaming; a future version could stream the markdown in chunks as sections are converted rather than converting the whole article before responding.

**Index freshness / incremental updates.** The current design is a full re-index on each dump update. An incremental path (diff two index files, re-embed only changed articles) is left for future work.


