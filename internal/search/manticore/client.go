package manticore

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tyrm/mcp-wikipedia-local/internal/search"
)

type Client struct {
	cfg *Config
	db  *sql.DB
}

func New(cfg *Config) (*Client, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Client{cfg: cfg, db: db}, nil
}

func (c *Client) CreateTable(ctx context.Context) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id bigint,
  title text stored indexed,
  lead_text text stored indexed,
  lang string,
  is_disambig bool,
  lead_embedding float_vector knn_dims='%d' hnsw_similarity='cosine'
) engine='columnar'`, c.cfg.Table, c.cfg.EmbedDims)
	_, err := c.db.ExecContext(ctx, q)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}

func (c *Client) TruncateTable(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, "TRUNCATE TABLE "+c.cfg.Table)
	if err != nil {
		return fmt.Errorf("truncate table: %w", err)
	}
	return nil
}

func (c *Client) BulkInsert(ctx context.Context, articles []search.Article) error {
	batchSize := c.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = len(articles)
	}

	for i := 0; i < len(articles); i += batchSize {
		end := min(i+batchSize, len(articles))
		if err := c.insertBatch(ctx, articles[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) insertBatch(ctx context.Context, articles []search.Article) error {
	if len(articles) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(c.cfg.Table)
	sb.WriteString(" (id,title,lead_text,lang,is_disambig,lead_embedding) VALUES ")

	args := make([]any, 0, len(articles)*6)
	for idx, a := range articles {
		if idx > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString("(?,?,?,?,?,?)")
		disambig := 0
		if a.IsDisambig {
			disambig = 1
		}
		args = append(args, a.ID, a.Title, a.LeadText, a.Lang, disambig, encodeVector(a.Embedding))
	}

	_, err := c.db.ExecContext(ctx, sb.String(), args...)
	if err != nil {
		return fmt.Errorf("bulk insert: %w", err)
	}
	return nil
}

func encodeVector(v []float32) string {
	if len(v) == 0 {
		return "()"
	}
	var sb strings.Builder
	sb.WriteByte('(')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(')')
	return sb.String()
}

func (c *Client) SearchBM25(ctx context.Context, query string, limit int) ([]search.Result, error) {
	q := fmt.Sprintf("SELECT id, title, lead_text, WEIGHT() as score FROM %s WHERE MATCH(?) ORDER BY score DESC LIMIT ?", c.cfg.Table)
	rows, err := c.db.QueryContext(ctx, q, query, limit)
	if err != nil {
		return nil, fmt.Errorf("bm25 query: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

func (c *Client) SearchKNN(ctx context.Context, embedding []float32, limit int) ([]search.Result, error) {
	vec := encodeVector(embedding)
	q := fmt.Sprintf("SELECT id, title, lead_text, KNN(lead_embedding, %d, ?) as score FROM %s ORDER BY score DESC LIMIT ?", limit, c.cfg.Table)
	rows, err := c.db.QueryContext(ctx, q, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("knn query: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

func (c *Client) SearchHybrid(ctx context.Context, query string, embedding []float32, limit int) ([]search.Result, error) {
	candidate := limit * 3

	var bm25Results, knnResults []search.Result
	var bm25Err, knnErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		bm25Results, bm25Err = c.SearchBM25(ctx, query, candidate)
	}()
	go func() {
		defer wg.Done()
		knnResults, knnErr = c.SearchKNN(ctx, embedding, candidate)
	}()
	wg.Wait()

	if bm25Err != nil {
		return nil, fmt.Errorf("hybrid bm25: %w", bm25Err)
	}
	if knnErr != nil {
		return nil, fmt.Errorf("hybrid knn: %w", knnErr)
	}

	return applyRRF(bm25Results, knnResults, limit), nil
}

func applyRRF(a, b []search.Result, limit int) []search.Result {
	scores := make(map[int64]float64)
	byID := make(map[int64]search.Result)

	for rank, r := range a {
		scores[r.ID] += 1.0 / (60.0 + float64(rank+1))
		byID[r.ID] = r
	}
	for rank, r := range b {
		scores[r.ID] += 1.0 / (60.0 + float64(rank+1))
		if _, ok := byID[r.ID]; !ok {
			byID[r.ID] = r
		}
	}

	type scored struct {
		id    int64
		score float64
	}
	all := make([]scored, 0, len(scores))
	for id, s := range scores {
		all = append(all, scored{id, s})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })

	if limit > len(all) {
		limit = len(all)
	}
	results := make([]search.Result, limit)
	for i := 0; i < limit; i++ {
		r := byID[all[i].id]
		r.Score = all[i].score
		results[i] = r
	}
	return results
}

func scanResults(rows *sql.Rows) ([]search.Result, error) {
	var results []search.Result
	for rows.Next() {
		var r search.Result
		if err := rows.Scan(&r.ID, &r.Title, &r.LeadSnippet, &r.Score); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return results, nil
}
