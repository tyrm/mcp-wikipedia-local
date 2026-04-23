package manticore

import (
	"math"
	"testing"

	"github.com/tyrm/mcp-wikipedia-local/internal/search"
)

func TestEncodeVector(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
		want  string
	}{
		{
			name:  "empty slice",
			input: []float32{},
			want:  "()",
		},
		{
			name:  "nil slice",
			input: nil,
			want:  "()",
		},
		{
			name:  "single element",
			input: []float32{1.5},
			want:  "(1.5)",
		},
		{
			name:  "multiple elements",
			input: []float32{0.1, 0.2, 0.3},
			want:  "(0.1,0.2,0.3)",
		},
		{
			name:  "negative values",
			input: []float32{-1.0, 2.5},
			want:  "(-1,2.5)",
		},
		{
			name:  "zero value",
			input: []float32{0},
			want:  "(0)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeVector(tc.input)
			if got != tc.want {
				t.Errorf("encodeVector(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestApplyRRF_BothEmpty(t *testing.T) {
	got := applyRRF(nil, nil, 10)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d items", len(got))
	}
}

func TestApplyRRF_SingleList(t *testing.T) {
	a := []search.Result{
		{ID: 1, Title: "First"},
		{ID: 2, Title: "Second"},
		{ID: 3, Title: "Third"},
	}

	got := applyRRF(a, nil, 3)

	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}

	want0 := 1.0 / (60.0 + 1.0)
	want1 := 1.0 / (60.0 + 2.0)
	want2 := 1.0 / (60.0 + 3.0)

	if math.Abs(got[0].Score-want0) > 1e-9 {
		t.Errorf("got[0].Score = %v, want %v", got[0].Score, want0)
	}
	if math.Abs(got[1].Score-want1) > 1e-9 {
		t.Errorf("got[1].Score = %v, want %v", got[1].Score, want1)
	}
	if math.Abs(got[2].Score-want2) > 1e-9 {
		t.Errorf("got[2].Score = %v, want %v", got[2].Score, want2)
	}
}

func TestApplyRRF_SameIDInBothListsHigherScore(t *testing.T) {
	shared := search.Result{ID: 42, Title: "Shared"}
	onlyA := search.Result{ID: 1, Title: "OnlyA"}
	onlyB := search.Result{ID: 2, Title: "OnlyB"}

	a := []search.Result{shared, onlyA}
	b := []search.Result{shared, onlyB}

	got := applyRRF(a, b, 10)

	var sharedScore, onlyAScore, onlyBScore float64
	for _, r := range got {
		switch r.ID {
		case 42:
			sharedScore = r.Score
		case 1:
			onlyAScore = r.Score
		case 2:
			onlyBScore = r.Score
		}
	}

	if sharedScore <= onlyAScore {
		t.Errorf("shared score %v should be greater than onlyA score %v", sharedScore, onlyAScore)
	}
	if sharedScore <= onlyBScore {
		t.Errorf("shared score %v should be greater than onlyB score %v", sharedScore, onlyBScore)
	}
}

func TestApplyRRF_LimitRespected(t *testing.T) {
	a := make([]search.Result, 20)
	for i := range a {
		a[i] = search.Result{ID: int64(i + 1), Title: "Title"}
	}

	got := applyRRF(a, nil, 5)
	if len(got) != 5 {
		t.Errorf("expected 5 results (limit), got %d", len(got))
	}
}

func TestApplyRRF_SortedDescending(t *testing.T) {
	a := []search.Result{
		{ID: 1, Title: "Rank1"},
		{ID: 2, Title: "Rank2"},
		{ID: 3, Title: "Rank3"},
	}
	b := []search.Result{
		{ID: 3, Title: "Rank3"},
		{ID: 1, Title: "Rank1"},
	}

	got := applyRRF(a, b, 10)

	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("results not sorted descending: got[%d].Score %v > got[%d].Score %v",
				i, got[i].Score, i-1, got[i-1].Score)
		}
	}
}

func TestApplyRRF_DeduplicationCombinedScore(t *testing.T) {
	r := search.Result{ID: 99, Title: "Dedup"}

	a := []search.Result{r}
	b := []search.Result{r}

	got := applyRRF(a, b, 10)

	count := 0
	for _, res := range got {
		if res.ID == 99 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected ID 99 to appear exactly once, got %d times", count)
	}
}

func TestApplyRRF_ScoreValueBothRank1(t *testing.T) {
	r := search.Result{ID: 1, Title: "Top"}

	a := []search.Result{r}
	b := []search.Result{r}

	got := applyRRF(a, b, 10)

	if len(got) == 0 {
		t.Fatal("expected at least one result")
	}

	wantScore := 1.0/61.0 + 1.0/61.0
	if math.Abs(got[0].Score-wantScore) > 1e-9 {
		t.Errorf("score = %v, want %v (~0.0328)", got[0].Score, wantScore)
	}
}

func TestApplyRRF_LimitLargerThanResults(t *testing.T) {
	a := []search.Result{
		{ID: 1, Title: "One"},
		{ID: 2, Title: "Two"},
	}

	got := applyRRF(a, nil, 100)
	if len(got) != 2 {
		t.Errorf("expected 2 results (capped to available), got %d", len(got))
	}
}
