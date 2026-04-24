package worker

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/schollz/progressbar/v3"
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

	// Pre-count checkpoint-skipped titles so the bar starts at the right position.
	var resumeAt int
	if len(checkpoint) > 0 {
		for _, t := range titles {
			if _, done := checkpoint[t]; done {
				resumeAt++
			}
		}
	}

	bar := progressbar.NewOptions(
		len(titles),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription("indexing"),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("articles"),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetWidth(40),
	)
	if resumeAt > 0 {
		_ = bar.Set(resumeAt)
	}

	titleCh := make(chan string, cfg.NumWorkers*2)
	resultCh := make(chan search.Article, cfg.NumWorkers*2)

	var idCounter atomic.Int64
	var indexed atomic.Int64

	var workerWg sync.WaitGroup
	for i := 0; i < cfg.NumWorkers; i++ {
		workerWg.Go(func() {
			for title := range titleCh {
				if _, ok := arch.OffsetForTitle(title); !ok {
					_ = bar.Add(1)
					continue
				}
				if _, alreadyDone := checkpoint[title]; alreadyDone {
					// already counted in resumeAt pre-pass, don't double-count
					continue
				}

				wikitext, err := arch.GetPage(title)
				if err != nil {
					zap.L().Error("get page", zap.String("title", title), zap.Error(err))
					_ = bar.Add(1)
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
				_ = bar.Add(1)
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
				titles := make([]string, len(batch))
				for i, a := range batch {
					titles[i] = a.Title
				}
				if appendErr := appendCheckpoint(cfg.CheckpointFile, titles); appendErr != nil {
					zap.L().Error("write checkpoint", zap.Error(appendErr))
				}
			}

			indexed.Add(int64(len(batch)))

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

func loadCheckpoint(path string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
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
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			set[line] = struct{}{}
		}
	}
	return set, scanner.Err()
}

func appendCheckpoint(path string, titles []string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, t := range titles {
		if _, err := fmt.Fprintln(w, t); err != nil {
			return err
		}
	}
	return w.Flush()
}
