package archive

import (
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
