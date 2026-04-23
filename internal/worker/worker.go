package worker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/tyrm/mcp-wikipedia-local/internal/archive"
	"github.com/tyrm/mcp-wikipedia-local/internal/embed"
	"github.com/tyrm/mcp-wikipedia-local/internal/parser"
	"github.com/tyrm/mcp-wikipedia-local/internal/search"
)

// Run runs the scan pipeline to completion or until ctx is cancelled.
// It returns the number of articles indexed and any fatal error.
func Run(ctx context.Context, arch *archive.Archive, embedClient embed.Client, searchClient search.Client, cfg *Config) (int64, error) {
	checkpoint, err := loadCheckpoint(cfg.CheckpointFile)
	if err != nil {
		return 0, fmt.Errorf("load checkpoint: %w", err)
	}

	titles := arch.Titles()

	titleCh := make(chan string, cfg.NumWorkers*2)
	resultCh := make(chan search.Article, cfg.NumWorkers*2)

	var idCounter atomic.Int64
	var indexed atomic.Int64

	var workerWg sync.WaitGroup
	for i := 0; i < cfg.NumWorkers; i++ {
		workerWg.Go(func() {
			for title := range titleCh {
				offset, ok := arch.OffsetForTitle(title)
				if !ok {
					continue
				}
				if _, alreadyDone := checkpoint[offset]; alreadyDone {
					continue
				}

				wikitext, err := arch.GetPage(title)
				if err != nil {
					zap.L().Error("get page", zap.String("title", title), zap.Error(err))
					continue
				}

				lead := parser.ExtractLead(wikitext)

				id := idCounter.Add(1)
				resultCh <- search.Article{
					ID:       id,
					Title:    title,
					LeadText: lead,
					Lang:     "en",
				}
			}
		})
	}

	go func() {
		workerWg.Wait()
		close(resultCh)
	}()

	var batchErr error

	batchDone := make(chan struct{})
	go func() {
		defer close(batchDone)
		batch := make([]search.Article, 0, cfg.BatchSize)

		flush := func() {
			if len(batch) == 0 {
				return
			}

			texts := make([]string, len(batch))
			for i, a := range batch {
				texts[i] = a.LeadText
			}

			embeddings, err := embedClient.Embed(ctx, texts)
			if err != nil {
				zap.L().Error("embed batch", zap.Error(err))
			} else if len(embeddings) == len(batch) {
				for i := range batch {
					batch[i].Embedding = embeddings[i]
				}
			}

			if err := searchClient.BulkInsert(ctx, batch); err != nil {
				zap.L().Error("bulk insert", zap.Error(err))
				batchErr = err
			}

			if cfg.CheckpointFile != "" {
				var offsets []int64
				for _, a := range batch {
					if off, ok := arch.OffsetForTitle(a.Title); ok {
						offsets = append(offsets, off)
					}
				}
				if appendErr := appendCheckpoint(cfg.CheckpointFile, offsets); appendErr != nil {
					zap.L().Error("write checkpoint", zap.Error(appendErr))
				}
			}

			n := indexed.Add(int64(len(batch)))
			if n/1000 > (n-int64(len(batch)))/1000 {
				zap.L().Info("indexing progress", zap.Int64("indexed", n))
			}

			batch = batch[:0]
		}

		for article := range resultCh {
			batch = append(batch, article)
			if len(batch) >= cfg.BatchSize {
				flush()
			}
		}
		flush()
	}()

	for _, t := range titles {
		select {
		case <-ctx.Done():
			close(titleCh)
			<-batchDone
			return indexed.Load(), batchErr
		case titleCh <- t:
		}
	}
	close(titleCh)

	<-batchDone

	return indexed.Load(), batchErr
}

func loadCheckpoint(path string) (map[int64]struct{}, error) {
	set := make(map[int64]struct{})
	if path == "" {
		return set, nil
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		offset, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			continue
		}
		set[offset] = struct{}{}
	}
	return set, scanner.Err()
}

func appendCheckpoint(path string, offsets []int64) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, o := range offsets {
		if _, err := fmt.Fprintln(w, o); err != nil {
			return err
		}
	}
	return w.Flush()
}
