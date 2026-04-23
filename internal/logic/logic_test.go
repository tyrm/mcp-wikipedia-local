package logic

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tyrm/mcp-wikipedia-local/internal/parser"
	"github.com/tyrm/mcp-wikipedia-local/internal/search"
)

// mockSearch implements search.Client.
type mockSearch struct {
	bm25Results    []search.Result
	bm25Err        error
	hybridResults  []search.Result
	hybridErr      error
	bm25CallCount  int
	hybridCallCount int
}

func (m *mockSearch) CreateTable(_ context.Context) error { return nil }
func (m *mockSearch) TruncateTable(_ context.Context) error { return nil }
func (m *mockSearch) BulkInsert(_ context.Context, _ []search.Article) error { return nil }
func (m *mockSearch) SearchKNN(_ context.Context, _ []float32, _ int) ([]search.Result, error) {
	return nil, nil
}
func (m *mockSearch) SearchBM25(_ context.Context, _ string, _ int) ([]search.Result, error) {
	m.bm25CallCount++
	return m.bm25Results, m.bm25Err
}
func (m *mockSearch) SearchHybrid(_ context.Context, _ string, _ []float32, _ int) ([]search.Result, error) {
	m.hybridCallCount++
	return m.hybridResults, m.hybridErr
}

// mockEmbed implements embed.Client.
type mockEmbed struct {
	embedResult [][]float32
	embedErr    error
}

func (m *mockEmbed) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return m.embedResult, m.embedErr
}
func (m *mockEmbed) Dims() int { return 384 }

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase words",
			input: "hello world",
			want:  "Hello World",
		},
		{
			name:  "multi word name",
			input: "albert einstein",
			want:  "Albert Einstein",
		},
		{
			name:  "already title case",
			input: "Albert Einstein",
			want:  "Albert Einstein",
		},
		{
			name:  "single word",
			input: "physics",
			want:  "Physics",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "single uppercase word",
			input: "Wikipedia",
			want:  "Wikipedia",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toTitleCase(tc.input)
			if got != tc.want {
				t.Errorf("toTitleCase(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPaginatePage(t *testing.T) {
	page := &parser.ParsedPage{
		Title: "Test Article",
		Lead:  "This is the lead paragraph.",
		Sections: []parser.Section{
			{Title: "History", Level: 2, Body: "### History\nHistory content here."},
			{Title: "Geography", Level: 2, Body: "### Geography\nGeography content here."},
			{Title: "Early Years", Level: 3, Body: "#### Early Years\nEarly years content."},
		},
		Outline: []parser.OutlineItem{
			{Title: "History", Level: 2, Index: 0},
			{Title: "Geography", Level: 2, Index: 1},
			{Title: "Early Years", Level: 3, Index: 2},
		},
	}

	chain := []string{"Redirect Source"}

	l := &Logic{cfg: &Config{DefaultLimit: 10}}

	t.Run("lead mode", func(t *testing.T) {
		resp := l.paginatePage(page, "lead", "", chain)
		if resp.Content != page.Lead {
			t.Errorf("lead mode Content = %q, want %q", resp.Content, page.Lead)
		}
		if resp.Title != "Test Article" {
			t.Errorf("Title = %q, want %q", resp.Title, "Test Article")
		}
		if len(resp.RedirectChain) != 1 || resp.RedirectChain[0] != "Redirect Source" {
			t.Errorf("RedirectChain = %v, want [Redirect Source]", resp.RedirectChain)
		}
	})

	t.Run("outline mode", func(t *testing.T) {
		resp := l.paginatePage(page, "outline", "", nil)
		if !strings.Contains(resp.Content, "History") {
			t.Errorf("outline mode should contain 'History', got: %q", resp.Content)
		}
		if !strings.Contains(resp.Content, "Geography") {
			t.Errorf("outline mode should contain 'Geography', got: %q", resp.Content)
		}
		if !strings.Contains(resp.Content, "Early Years") {
			t.Errorf("outline mode should contain 'Early Years', got: %q", resp.Content)
		}
	})

	t.Run("section mode matching title", func(t *testing.T) {
		resp := l.paginatePage(page, "section", "History", nil)
		if resp.Content != page.Sections[0].Body {
			t.Errorf("section mode Content = %q, want %q", resp.Content, page.Sections[0].Body)
		}
	})

	t.Run("section mode case-insensitive", func(t *testing.T) {
		resp := l.paginatePage(page, "section", "history", nil)
		if resp.Content != page.Sections[0].Body {
			t.Errorf("section mode case-insensitive Content = %q, want %q", resp.Content, page.Sections[0].Body)
		}
	})

	t.Run("section mode no match", func(t *testing.T) {
		resp := l.paginatePage(page, "section", "Nonexistent", nil)
		if resp.Content != "" {
			t.Errorf("non-matching section should return empty content, got %q", resp.Content)
		}
	})

	t.Run("full mode", func(t *testing.T) {
		resp := l.paginatePage(page, "full", "", nil)
		if !strings.Contains(resp.Content, page.Lead) {
			t.Errorf("full mode should contain lead, got: %q", resp.Content)
		}
		if !strings.Contains(resp.Content, page.Sections[0].Body) {
			t.Errorf("full mode should contain first section body, got: %q", resp.Content)
		}
		if !strings.Contains(resp.Content, page.Sections[1].Body) {
			t.Errorf("full mode should contain second section body, got: %q", resp.Content)
		}
	})

	t.Run("empty mode same as full", func(t *testing.T) {
		full := l.paginatePage(page, "full", "", nil)
		empty := l.paginatePage(page, "", "", nil)
		if full.Content != empty.Content {
			t.Errorf("empty mode content %q != full mode content %q", empty.Content, full.Content)
		}
	})

	t.Run("redirect chain propagated", func(t *testing.T) {
		theChain := []string{"A", "B"}
		resp := l.paginatePage(page, "lead", "", theChain)
		if len(resp.RedirectChain) != 2 {
			t.Errorf("expected redirect chain length 2, got %d", len(resp.RedirectChain))
		}
	})

	t.Run("outline level markers", func(t *testing.T) {
		resp := l.paginatePage(page, "outline", "", nil)
		for line := range strings.SplitSeq(strings.TrimSpace(resp.Content), "\n") {
			if strings.Contains(line, "History") && !strings.HasPrefix(line, "##") {
				t.Errorf("History outline line should start with ##, got: %q", line)
			}
		}
	})
}

func TestSearch_EmbedSuccessUsesHybrid(t *testing.T) {
	ms := &mockSearch{
		hybridResults: []search.Result{{ID: 1, Title: "Result One", Score: 0.9}},
	}
	me := &mockEmbed{
		embedResult: [][]float32{{0.1, 0.2, 0.3}},
	}

	l := &Logic{cfg: &Config{
		Search:       ms,
		Embed:        me,
		DefaultLimit: 10,
	}}

	results, err := l.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms.hybridCallCount != 1 {
		t.Errorf("expected SearchHybrid called once, got %d", ms.hybridCallCount)
	}
	if ms.bm25CallCount != 0 {
		t.Errorf("expected SearchBM25 not called, got %d times", ms.bm25CallCount)
	}
	if len(results) != 1 || results[0].ID != 1 {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestSearch_EmbedFailsFallsBackToBM25(t *testing.T) {
	ms := &mockSearch{
		bm25Results: []search.Result{{ID: 2, Title: "BM25 Result", Score: 0.5}},
	}
	me := &mockEmbed{
		embedErr: errors.New("embedding service unavailable"),
	}

	l := &Logic{cfg: &Config{
		Search:       ms,
		Embed:        me,
		DefaultLimit: 10,
	}}

	results, err := l.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms.hybridCallCount != 0 {
		t.Errorf("expected SearchHybrid not called, got %d times", ms.hybridCallCount)
	}
	if ms.bm25CallCount != 1 {
		t.Errorf("expected SearchBM25 called once, got %d", ms.bm25CallCount)
	}
	if len(results) != 1 || results[0].ID != 2 {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestSearch_HybridFailsFallsBackToBM25(t *testing.T) {
	ms := &mockSearch{
		hybridErr:   errors.New("hybrid search failed"),
		bm25Results: []search.Result{{ID: 3, Title: "Fallback", Score: 0.3}},
	}
	me := &mockEmbed{
		embedResult: [][]float32{{0.1, 0.2}},
	}

	l := &Logic{cfg: &Config{
		Search:       ms,
		Embed:        me,
		DefaultLimit: 10,
	}}

	results, err := l.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms.hybridCallCount != 1 {
		t.Errorf("expected SearchHybrid called once, got %d", ms.hybridCallCount)
	}
	if ms.bm25CallCount != 1 {
		t.Errorf("expected SearchBM25 called once for fallback, got %d", ms.bm25CallCount)
	}
	if len(results) != 1 || results[0].ID != 3 {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestSearch_DefaultLimitApplied(t *testing.T) {
	capturedLimit := 0
	ms := &mockSearchCapture{
		onBM25: func(_ string, limit int) ([]search.Result, error) {
			capturedLimit = limit
			return nil, nil
		},
		onHybrid: func(_ string, _ []float32, limit int) ([]search.Result, error) {
			capturedLimit = limit
			return nil, nil
		},
	}
	me := &mockEmbed{
		embedResult: [][]float32{{0.1}},
	}

	l := &Logic{cfg: &Config{
		Search:       ms,
		Embed:        me,
		DefaultLimit: 42,
	}}

	_, _ = l.Search(context.Background(), "query", 0)

	if capturedLimit != 42 {
		t.Errorf("expected default limit 42, got %d", capturedLimit)
	}
}

// mockSearchCapture allows capturing call arguments.
type mockSearchCapture struct {
	onBM25   func(query string, limit int) ([]search.Result, error)
	onHybrid func(query string, embedding []float32, limit int) ([]search.Result, error)
}

func (m *mockSearchCapture) CreateTable(_ context.Context) error  { return nil }
func (m *mockSearchCapture) TruncateTable(_ context.Context) error { return nil }
func (m *mockSearchCapture) BulkInsert(_ context.Context, _ []search.Article) error { return nil }
func (m *mockSearchCapture) SearchKNN(_ context.Context, _ []float32, _ int) ([]search.Result, error) {
	return nil, nil
}
func (m *mockSearchCapture) SearchBM25(_ context.Context, query string, limit int) ([]search.Result, error) {
	if m.onBM25 != nil {
		return m.onBM25(query, limit)
	}
	return nil, nil
}
func (m *mockSearchCapture) SearchHybrid(_ context.Context, query string, embedding []float32, limit int) ([]search.Result, error) {
	if m.onHybrid != nil {
		return m.onHybrid(query, embedding, limit)
	}
	return nil, nil
}
