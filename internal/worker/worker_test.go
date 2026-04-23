package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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

func TestLoadCheckpoint_ValidOffsets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	content := "100\n200\n300\n"
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

	for _, offset := range []int64{100, 200, 300} {
		if _, ok := got[offset]; !ok {
			t.Errorf("expected offset %d to be in map", offset)
		}
	}
}

func TestLoadCheckpoint_MixedValidInvalidLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	content := "100\nnot-a-number\n200\n\n300\nbad\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 valid entries, got %d", len(got))
	}
	for _, offset := range []int64{100, 200, 300} {
		if _, ok := got[offset]; !ok {
			t.Errorf("expected offset %d to be in map", offset)
		}
	}
}

func TestAppendCheckpoint_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new_checkpoint.txt")

	if err := appendCheckpoint(path, []int64{111, 222}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestAppendCheckpoint_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	if err := appendCheckpoint(path, []int64{100, 200}); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := appendCheckpoint(path, []int64{300, 400}); err != nil {
		t.Fatalf("second append: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 entries after two appends, got %d", len(got))
	}
	for _, offset := range []int64{100, 200, 300, 400} {
		if _, ok := got[offset]; !ok {
			t.Errorf("expected offset %d in checkpoint", offset)
		}
	}
}

func TestAppendCheckpoint_EmptyOffsets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.txt")

	if err := appendCheckpoint(path, []int64{}); err != nil {
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

	offsets := []int64{1024, 2048, 4096, 8192, 16384}

	if err := appendCheckpoint(path, offsets); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != len(offsets) {
		t.Fatalf("expected %d entries, got %d", len(offsets), len(got))
	}
	for _, o := range offsets {
		if _, ok := got[o]; !ok {
			t.Errorf("offset %d not found after round-trip", o)
		}
	}
}

func TestAppendCheckpoint_LargeOffsets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	var offsets []int64
	for i := range 1000 {
		offsets = append(offsets, int64(i)*512)
	}

	if err := appendCheckpoint(path, offsets); err != nil {
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

	content := "  100  \n\t200\t\n300\n"
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

	allOffsets := make(map[int64]struct{})
	for batch := range 5 {
		var offsets []int64
		for i := range 10 {
			o := int64(batch*1000 + i)
			offsets = append(offsets, o)
			allOffsets[o] = struct{}{}
		}
		if err := appendCheckpoint(path, offsets); err != nil {
			t.Fatalf("batch %d append: %v", batch, err)
		}
	}

	got, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != len(allOffsets) {
		t.Errorf("expected %d entries, got %d", len(allOffsets), len(got))
	}
	for o := range allOffsets {
		if _, ok := got[o]; !ok {
			t.Errorf("offset %d missing after multi-batch round-trip", o)
		}
	}
}

func TestLoadCheckpoint_DuplicateOffsets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dupes.txt")

	content := fmt.Sprintf("%d\n%d\n%d\n", 100, 100, 200)
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
