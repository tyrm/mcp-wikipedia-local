package parser

import (
	"strings"
	"testing"
)

func TestParsePage_Redirect(t *testing.T) {
	tests := []struct {
		name     string
		wikitext string
		want     string
	}{
		{
			name:     "basic redirect",
			wikitext: "#REDIRECT [[Target Article]]",
			want:     "Target Article",
		},
		{
			name:     "redirect with anchor",
			wikitext: "#REDIRECT [[Target Article|anchor text]]",
			want:     "Target Article",
		},
		{
			name:     "lowercase redirect",
			wikitext: "#redirect [[Lowercase Target]]",
			want:     "Lowercase Target",
		},
		{
			name:     "mixed case redirect",
			wikitext: "#Redirect [[Mixed Case]]",
			want:     "Mixed Case",
		},
		{
			name:     "redirect with leading whitespace on target",
			wikitext: "#REDIRECT [[ Spaced Target]]",
			want:     "Spaced Target",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParsePage("TestTitle", tc.wikitext)
			if p.IsRedirect != tc.want {
				t.Errorf("IsRedirect = %q, want %q", p.IsRedirect, tc.want)
			}
			if len(p.Sections) != 0 {
				t.Errorf("expected no sections on redirect, got %d", len(p.Sections))
			}
		})
	}
}

func TestParsePage_Disambiguation(t *testing.T) {
	tests := []struct {
		name       string
		wikitext   string
		wantDisamb bool
	}{
		{
			name:       "disambiguation template",
			wikitext:   "Some text\n{{disambiguation}}\nMore text",
			wantDisamb: true,
		},
		{
			name:       "disambig short form",
			wikitext:   "{{disambig}}",
			wantDisamb: true,
		},
		{
			name:       "uppercase disambiguation",
			wikitext:   "{{Disambiguation}}",
			wantDisamb: true,
		},
		{
			name:       "normal page no disambig",
			wikitext:   "This is a normal article about something.",
			wantDisamb: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParsePage("TestTitle", tc.wikitext)
			if p.IsDisambig != tc.wantDisamb {
				t.Errorf("IsDisambig = %v, want %v", p.IsDisambig, tc.wantDisamb)
			}
		})
	}
}

func TestParsePage_NormalPage(t *testing.T) {
	wikitext := `Albert Einstein was a German-born theoretical physicist.

== Early life ==
Einstein was born on 14 March 1879.

=== Childhood ===
He grew up in Munich.

== Career ==
He developed the theory of relativity.`

	p := ParsePage("Albert Einstein", wikitext)

	if p.Title != "Albert Einstein" {
		t.Errorf("Title = %q, want %q", p.Title, "Albert Einstein")
	}
	if p.IsRedirect != "" {
		t.Errorf("IsRedirect should be empty, got %q", p.IsRedirect)
	}
	if !strings.Contains(p.Lead, "German-born theoretical physicist") {
		t.Errorf("Lead does not contain expected text, got: %q", p.Lead)
	}

	if len(p.Sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(p.Sections))
	} else {
		if p.Sections[0].Title != "Early life" {
			t.Errorf("Sections[0].Title = %q, want %q", p.Sections[0].Title, "Early life")
		}
		if p.Sections[0].Level != 2 {
			t.Errorf("Sections[0].Level = %d, want 2", p.Sections[0].Level)
		}
		if p.Sections[1].Title != "Childhood" {
			t.Errorf("Sections[1].Title = %q, want %q", p.Sections[1].Title, "Childhood")
		}
		if p.Sections[1].Level != 3 {
			t.Errorf("Sections[1].Level = %d, want 3", p.Sections[1].Level)
		}
		if p.Sections[2].Title != "Career" {
			t.Errorf("Sections[2].Title = %q, want %q", p.Sections[2].Title, "Career")
		}
	}

	if len(p.Outline) != 3 {
		t.Errorf("expected 3 outline items, got %d", len(p.Outline))
	}
}

func TestParsePage_SectionLevels(t *testing.T) {
	wikitext := `Lead text.

== H2 Section ==
H2 content.

=== H3 Section ===
H3 content.`

	p := ParsePage("Test", wikitext)

	if len(p.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(p.Sections))
	}

	if p.Sections[0].Level != 2 {
		t.Errorf("first section level = %d, want 2", p.Sections[0].Level)
	}
	if p.Sections[1].Level != 3 {
		t.Errorf("second section level = %d, want 3", p.Sections[1].Level)
	}
}

func TestParsePage_BoldItalicConversion(t *testing.T) {
	wikitext := `'''Bold text''' and ''italic text'' in the lead.

== Section ==
'''Bold''' and ''italic'' in a section.`

	p := ParsePage("Test", wikitext)

	if !strings.Contains(p.Lead, "**Bold text**") {
		t.Errorf("Lead should contain **Bold text**, got: %q", p.Lead)
	}
	if !strings.Contains(p.Lead, "*italic text*") {
		t.Errorf("Lead should contain *italic text*, got: %q", p.Lead)
	}
	if !strings.Contains(p.Sections[0].Body, "**Bold**") {
		t.Errorf("Section body should contain **Bold**, got: %q", p.Sections[0].Body)
	}
	if !strings.Contains(p.Sections[0].Body, "*italic*") {
		t.Errorf("Section body should contain *italic*, got: %q", p.Sections[0].Body)
	}
}

func TestParsePage_WikilinkConversion(t *testing.T) {
	wikitext := `See [[Article]] and [[Article|display text]] for more.`

	p := ParsePage("Test", wikitext)

	if !strings.Contains(p.Lead, "[Article](Article)") {
		t.Errorf("Lead should contain [Article](Article), got: %q", p.Lead)
	}
	if !strings.Contains(p.Lead, "[display text](Article)") {
		t.Errorf("Lead should contain [display text](Article), got: %q", p.Lead)
	}
}

func TestParsePage_ExternalLinkConversion(t *testing.T) {
	wikitext := `Visit [http://example.com Example Site] for more info.`

	p := ParsePage("Test", wikitext)

	if !strings.Contains(p.Lead, "[Example Site](http://example.com)") {
		t.Errorf("Lead should contain markdown external link, got: %q", p.Lead)
	}
}

func TestParsePage_RefTagStripping(t *testing.T) {
	wikitext := `Some text<ref>This is a reference.</ref> and more<ref name="named"/> text.`

	p := ParsePage("Test", wikitext)

	if strings.Contains(p.Lead, "<ref") {
		t.Errorf("Lead should not contain ref tags, got: %q", p.Lead)
	}
	if strings.Contains(p.Lead, "This is a reference") {
		t.Errorf("Lead should not contain ref content, got: %q", p.Lead)
	}
}

func TestParsePage_InfoboxStripping(t *testing.T) {
	wikitext := `{{Infobox person
| name = Albert Einstein
| birth_date = 1879-03-14
}}
Albert Einstein was a physicist.`

	p := ParsePage("Test", wikitext)

	if strings.Contains(p.Lead, "Infobox") {
		t.Errorf("Lead should not contain infobox, got: %q", p.Lead)
	}
	if strings.Contains(p.Lead, "birth_date") {
		t.Errorf("Lead should not contain infobox fields, got: %q", p.Lead)
	}
	if !strings.Contains(p.Lead, "Albert Einstein was a physicist") {
		t.Errorf("Lead should contain text after infobox, got: %q", p.Lead)
	}
}

func TestParsePage_CategoryFileImageStripped(t *testing.T) {
	wikitext := `Normal lead text.
[[Category:German physicists]]
[[File:Einstein.jpg|thumb|Caption]]
[[Image:Photo.png|Einstein]]`

	p := ParsePage("Test", wikitext)

	if strings.Contains(p.Lead, "Category:") {
		t.Errorf("Lead should not contain category lines, got: %q", p.Lead)
	}
	if strings.Contains(p.Lead, "File:") {
		t.Errorf("Lead should not contain file lines, got: %q", p.Lead)
	}
	if strings.Contains(p.Lead, "Image:") {
		t.Errorf("Lead should not contain image lines, got: %q", p.Lead)
	}
	if !strings.Contains(p.Lead, "Normal lead text") {
		t.Errorf("Lead should contain normal text, got: %q", p.Lead)
	}
}

func TestExtractLead(t *testing.T) {
	tests := []struct {
		name     string
		wikitext string
		want     string
	}{
		{
			name:     "text before heading",
			wikitext: "This is the lead.\n\n== Section ==\nSection content.",
			want:     "This is the lead.",
		},
		{
			name:     "stops at heading",
			wikitext: "Lead text.\n== Heading ==\nAfter heading.",
			want:     "Lead text.",
		},
		{
			name:     "applies inline conversions",
			wikitext: "See [[Article|display]] and '''bold'''.",
			want:     "See [display](Article) and **bold**.",
		},
		{
			name:     "empty wikitext",
			wikitext: "",
			want:     "",
		},
		{
			name:     "no heading returns all text",
			wikitext: "Just a lead with no sections.",
			want:     "Just a lead with no sections.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractLead(tc.wikitext)
			if got != tc.want {
				t.Errorf("ExtractLead() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConvertWikilinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple wikilink",
			input: "[[A]]",
			want:  "[A](A)",
		},
		{
			name:  "wikilink with display text",
			input: "[[A|B]]",
			want:  "[B](A)",
		},
		{
			name:  "unclosed bracket no panic",
			input: "[[Unclosed",
			want:  "[[Unclosed",
		},
		{
			name:  "multiple wikilinks",
			input: "See [[First]] and [[Second|display]].",
			want:  "See [First](First) and [display](Second).",
		},
		{
			name:  "no wikilinks unchanged",
			input: "Plain text with no links.",
			want:  "Plain text with no links.",
		},
		{
			name:  "wikilink adjacent to text",
			input: "text[[A]]text",
			want:  "text[A](A)text",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := convertWikilinks(tc.input)
			if got != tc.want {
				t.Errorf("convertWikilinks(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStripTables(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "basic table stripped",
			input:       "Before\n{| class=\"wikitable\"\n| Cell\n|}\nAfter",
			wantContain: "Before",
			wantAbsent:  "Cell",
		},
		{
			name:        "preserves text before table",
			input:       "Text before.\n{|\n| Row\n|}\n",
			wantContain: "Text before.",
			wantAbsent:  "Row",
		},
		{
			name:        "preserves text after table",
			input:       "{|\n| Row\n|}\nText after.",
			wantContain: "Text after.",
			wantAbsent:  "Row",
		},
		{
			name:        "nested tables stripped",
			input:       "Before\n{|\n| outer\n{|\n| inner\n|}\n|}\nAfter",
			wantContain: "After",
			wantAbsent:  "inner",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripTables(tc.input)
			if !strings.Contains(got, tc.wantContain) {
				t.Errorf("stripTables() = %q, want it to contain %q", got, tc.wantContain)
			}
			if strings.Contains(got, tc.wantAbsent) {
				t.Errorf("stripTables() = %q, should not contain %q", got, tc.wantAbsent)
			}
		})
	}
}
