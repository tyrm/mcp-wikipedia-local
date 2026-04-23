package archive

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const minimalWikiXML = `<mediawiki>
  <page>
    <title>Main Article</title>
    <ns>0</ns>
    <revision>
      <text>This is the main article wikitext.</text>
    </revision>
  </page>
  <page>
    <title>Talk:Discussion</title>
    <ns>1</ns>
    <revision>
      <text>Talk page content.</text>
    </revision>
  </page>
  <page>
    <title>Second Article</title>
    <ns>0</ns>
    <revision>
      <text>Second article wikitext.</text>
    </revision>
  </page>
  <page>
    <title>Wikipedia:Policy</title>
    <ns>4</ns>
    <revision>
      <text>Policy page content.</text>
    </revision>
  </page>
</mediawiki>`

func TestExtractPage_Found(t *testing.T) {
	r := strings.NewReader(minimalWikiXML)
	got, err := extractPage(r, "Main Article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "This is the main article wikitext." {
		t.Errorf("extractPage() = %q, want %q", got, "This is the main article wikitext.")
	}
}

func TestExtractPage_SecondArticle(t *testing.T) {
	r := strings.NewReader(minimalWikiXML)
	got, err := extractPage(r, "Second Article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Second article wikitext." {
		t.Errorf("extractPage() = %q, want %q", got, "Second article wikitext.")
	}
}

func TestExtractPage_SkipsNonMainNamespace(t *testing.T) {
	r := strings.NewReader(minimalWikiXML)
	_, err := extractPage(r, "Talk:Discussion")
	if err == nil {
		t.Error("expected error for non-main-namespace title, got nil")
	}
}

func TestExtractPage_SkipsNS4(t *testing.T) {
	r := strings.NewReader(minimalWikiXML)
	_, err := extractPage(r, "Wikipedia:Policy")
	if err == nil {
		t.Error("expected error for NS=4 page, got nil")
	}
}

func TestExtractPage_NotFound(t *testing.T) {
	r := strings.NewReader(minimalWikiXML)
	_, err := extractPage(r, "Nonexistent Article")
	if err == nil {
		t.Fatal("expected error for missing title, got nil")
	}
	if !strings.Contains(err.Error(), "page not found in stream") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "page not found in stream")
	}
}

func TestExtractPage_EmptyXML(t *testing.T) {
	r := strings.NewReader("<mediawiki></mediawiki>")
	_, err := extractPage(r, "Any Title")
	if err == nil {
		t.Fatal("expected error for empty XML, got nil")
	}
}

func TestExtractPage_MultipleNS0Pages(t *testing.T) {
	xml := `<mediawiki>
  <page><title>Alpha</title><ns>0</ns><revision><text>alpha text</text></revision></page>
  <page><title>Beta</title><ns>0</ns><revision><text>beta text</text></revision></page>
  <page><title>Gamma</title><ns>0</ns><revision><text>gamma text</text></revision></page>
</mediawiki>`

	tests := []struct {
		title string
		want  string
	}{
		{"Alpha", "alpha text"},
		{"Beta", "beta text"},
		{"Gamma", "gamma text"},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			r := strings.NewReader(xml)
			got, err := extractPage(r, tc.title)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("extractPage(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

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

func TestLoadIndex_ThreeEntries(t *testing.T) {
	dir := t.TempDir()

	archiveData := bzip2Compress(t, []byte{})
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, archiveData, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexContent := "0:1:Anarchism\n627:2:Albert Einstein\n1254:3:Aardvark\n"
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := New(&Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	titles := arch.Titles()
	if len(titles) != 3 {
		t.Errorf("Titles() len = %d, want 3", len(titles))
	}
}

func TestOffsetForTitle_Found(t *testing.T) {
	dir := t.TempDir()

	archiveData := bzip2Compress(t, []byte{})
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, archiveData, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexContent := "0:1:Anarchism\n627:2:Albert Einstein\n1254:3:Aardvark\n"
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := New(&Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	offset, ok := arch.OffsetForTitle("Anarchism")
	if !ok {
		t.Fatal("OffsetForTitle(\"Anarchism\") ok = false, want true")
	}
	if offset != 0 {
		t.Errorf("OffsetForTitle(\"Anarchism\") = %d, want 0", offset)
	}
}

func TestOffsetForTitle_Missing(t *testing.T) {
	dir := t.TempDir()

	archiveData := bzip2Compress(t, []byte{})
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, archiveData, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexContent := "0:1:Anarchism\n"
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := New(&Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	offset, ok := arch.OffsetForTitle("Missing")
	if ok {
		t.Error("OffsetForTitle(\"Missing\") ok = true, want false")
	}
	if offset != 0 {
		t.Errorf("OffsetForTitle(\"Missing\") = %d, want 0", offset)
	}
}

func TestOffsetForTitle_TitleWithColon(t *testing.T) {
	dir := t.TempDir()

	archiveData := bzip2Compress(t, []byte{})
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, archiveData, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexContent := "0:1:Anarchism\n627:2:Albert Einstein\n1254:3:Aardvark\n1881:4:Talk:Foo\n"
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := New(&Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	offset, ok := arch.OffsetForTitle("Talk:Foo")
	if !ok {
		t.Fatal("OffsetForTitle(\"Talk:Foo\") ok = false, want true")
	}
	if offset != 1881 {
		t.Errorf("OffsetForTitle(\"Talk:Foo\") = %d, want 1881", offset)
	}
}

func TestGetPage_MultiStream(t *testing.T) {
	dir := t.TempDir()

	stream0XML := `<page><title>Anarchism</title><ns>0</ns><revision><text xml:space="preserve">Anarchism is a political philosophy.</text></revision></page><page><title>Aardvark</title><ns>0</ns><revision><text xml:space="preserve">The aardvark is a mammal.</text></revision></page>`
	stream1XML := `<page><title>Albert Einstein</title><ns>0</ns><revision><text xml:space="preserve">Albert Einstein was a physicist.</text></revision></page>`

	stream0 := bzip2Compress(t, []byte(stream0XML))
	stream1 := bzip2Compress(t, []byte(stream1XML))

	archiveData := append(stream0, stream1...)
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, archiveData, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	stream0Len := int64(len(stream0))
	indexContent := fmt.Sprintf("0:1:Anarchism\n0:2:Aardvark\n%d:3:Albert Einstein\n", stream0Len)
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := New(&Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	tests := []struct {
		title   string
		wantSub string
	}{
		{"Anarchism", "political philosophy"},
		{"Aardvark", "mammal"},
		{"Albert Einstein", "physicist"},
	}

	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			got, err := arch.GetPage(tc.title)
			if err != nil {
				t.Fatalf("GetPage(%q): %v", tc.title, err)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("GetPage(%q) = %q, want it to contain %q", tc.title, got, tc.wantSub)
			}
		})
	}
}

func TestGetPage_Missing(t *testing.T) {
	dir := t.TempDir()

	stream0XML := `<page><title>Anarchism</title><ns>0</ns><revision><text xml:space="preserve">Anarchism is a political philosophy.</text></revision></page>`
	stream0 := bzip2Compress(t, []byte(stream0XML))
	archivePath := filepath.Join(dir, "archive.bz2")
	if err := os.WriteFile(archivePath, stream0, 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	indexContent := "0:1:Anarchism\n"
	indexData := bzip2Compress(t, []byte(indexContent))
	indexPath := filepath.Join(dir, "index.bz2")
	if err := os.WriteFile(indexPath, indexData, 0600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	arch, err := New(&Config{Path: archivePath, IndexPath: indexPath})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer arch.Close()

	if err := arch.LoadIndex(); err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	_, err = arch.GetPage("Missing")
	if err == nil {
		t.Fatal("GetPage(\"Missing\") expected error, got nil")
	}
}
