package worker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tyrm/mcp-wikipedia-local/internal/archive"
	"github.com/tyrm/mcp-wikipedia-local/internal/search"
	"github.com/tyrm/mcp-wikipedia-local/internal/search/manticore"
)

func TestLoadCheckpoint_EmptyPath(t *testing.T) {
	got, err := loadCheckpoint("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestLoadCheckpoint_NonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.txt")
	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestLoadCheckpoint_ValidTitles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	content := "Anarchism\nAlbert Einstein\nAardvark\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	for _, title := range []string{"Anarchism", "Albert Einstein", "Aardvark"} {
		if _, ok := got[title]; !ok {
			t.Errorf("expected title %q to be in map", title)
		}
	}
}

func TestLoadCheckpoint_EmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	content := "Anarchism\n\nAlbert Einstein\n\n\nAardvark\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 entries (empty lines skipped), got %d", len(got))
	}
}

func TestAppendCheckpoint_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new_checkpoint.txt")

	if err := appendCheckpoint(path, []string{"Anarchism", "Aardvark"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestAppendCheckpoint_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	if err := appendCheckpoint(path, []string{"Anarchism", "Aardvark"}); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := appendCheckpoint(path, []string{"Albert Einstein", "2001: A Space Odyssey"}); err != nil {
		t.Fatalf("second append: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 entries after two appends, got %d", len(got))
	}
	for _, title := range []string{"Anarchism", "Aardvark", "Albert Einstein", "2001: A Space Odyssey"} {
		if _, ok := got[title]; !ok {
			t.Errorf("expected title %q in checkpoint", title)
		}
	}
}

func TestAppendCheckpoint_EmptySlice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	if err := appendCheckpoint(path, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat error: %v", err)
	}
	if err == nil && info.Size() != 0 {
		t.Errorf("expected empty file, got size %d", info.Size())
	}
}

func TestAppendCheckpoint_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.txt")

	titles := []string{"Anarchism", "Albert Einstein", "Aardvark", "2001: A Space Odyssey", "Go (programming language)"}

	if err := appendCheckpoint(path, titles); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != len(titles) {
		t.Fatalf("expected %d entries, got %d", len(titles), len(got))
	}
	for _, title := range titles {
		if _, ok := got[title]; !ok {
			t.Errorf("title %q not found after round-trip", title)
		}
	}
}

func TestAppendCheckpoint_LargeBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	titles := make([]string, 1000)
	for i := range 1000 {
		titles[i] = fmt.Sprintf("Article %d", i)
	}

	if err := appendCheckpoint(path, titles); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1000 {
		t.Errorf("expected 1000 entries, got %d", len(got))
	}
}

func TestLoadCheckpoint_WhitespaceHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	content := "  Anarchism  \n\tAlbert Einstein\t\nAardvark\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(got), got)
	}
}

func TestAppendCheckpoint_MultipleRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")

	allTitles := make(map[string]struct{})
	for batch := range 5 {
		var titles []string
		for i := range 10 {
			title := fmt.Sprintf("Article %d-%d", batch, i)
			titles = append(titles, title)
			allTitles[title] = struct{}{}
		}
		if err := appendCheckpoint(path, titles); err != nil {
			t.Fatalf("batch %d append: %v", batch, err)
		}
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != len(allTitles) {
		t.Errorf("expected %d entries, got %d", len(allTitles), len(got))
	}
	for title := range allTitles {
		if _, ok := got[title]; !ok {
			t.Errorf("title %q missing after multi-batch round-trip", title)
		}
	}
}

func TestLoadCheckpoint_DuplicateTitles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dupes.txt")

	content := "Anarchism\nAnarchism\nAardvark\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 unique entries (deduped), got %d", len(got))
	}
}

type fakeEmbedClient struct{ dims int }

func (f *fakeEmbedClient) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, f.dims)
	}
	return out, nil
}

func (f *fakeEmbedClient) Dims() int { return f.dims }

type fakeSearchClient struct{}

func (f *fakeSearchClient) CreateTable(_ context.Context) error { return nil }
func (f *fakeSearchClient) BulkInsert(_ context.Context, _ []search.Article) error {
	return nil
}
func (f *fakeSearchClient) SearchBM25(_ context.Context, _ string, _ int) ([]search.Result, error) {
	return nil, nil
}
func (f *fakeSearchClient) SearchKNN(_ context.Context, _ []float32, _ int) ([]search.Result, error) {
	return nil, nil
}
func (f *fakeSearchClient) SearchHybrid(_ context.Context, _ string, _ []float32, _ int) ([]search.Result, error) {
	return nil, nil
}
func (f *fakeSearchClient) TruncateTable(_ context.Context) error { return nil }

func bzip2Compress(t *testing.T, data []byte) []byte {
	t.Helper()
	cmd := exec.Command("bzip2", "-c")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		t.Skip("bzip2 not available")
	}
	return out
}

func TestRun_ContextCancellation(t *testing.T) {
	dir := t.TempDir()

	archiveData := bzip2Compress(t, []byte{})
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, archiveData, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexData := bzip2Compress(t, []byte{})
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := archive.New(&archive.Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("archive.New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	count, err := Run(ctx, arch, &fakeEmbedClient{dims: 768}, &fakeSearchClient{}, &Config{
		NumWorkers: 2,
		BatchSize:  10,
	})
	if err != nil && err != context.Canceled {
		t.Errorf("Run returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0 for empty index, got %d", count)
	}
}

func manticoreDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("MANTICORE_DSN")
	if dsn == "" {
		t.Skip("MANTICORE_DSN not set")
	}
	return dsn
}

func TestIntegration_Run(t *testing.T) {
	dsn := manticoreDSN(t)
	dir := t.TempDir()

	stream0XML := `<page><title>Anarchism</title><ns>0</ns><revision><text xml:space="preserve">Anarchism is a political philosophy.</text></revision></page><page><title>Aardvark</title><ns>0</ns><revision><text xml:space="preserve">The aardvark is a mammal.</text></revision></page>`
	stream0 := bzip2Compress(t, []byte(stream0XML))

	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, stream0, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexContent := "0:1:Anarchism\n0:2:Aardvark\n"
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := archive.New(&archive.Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("archive.New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	table := fmt.Sprintf("wiki_test_%d", time.Now().UnixNano())
	sc, err := manticore.New(&manticore.Config{DSN: dsn, Table: table, EmbedDims: 768})
	if err != nil {
		t.Fatalf("manticore.New: %v", err)
	}

	ctx := context.Background()
	if err := sc.CreateTable(ctx); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	count, err := Run(ctx, arch, &fakeEmbedClient{dims: 768}, sc, &Config{
		NumWorkers: 2,
		BatchSize:  2,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if count != 2 {
		t.Errorf("Run count = %d, want 2", count)
	}

	results, err := sc.SearchBM25(ctx, "political", 10)
	if err != nil {
		t.Fatalf("SearchBM25: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 search result after indexing")
	}
}
