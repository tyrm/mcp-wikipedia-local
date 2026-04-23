package parser

import (
	"regexp"
	"strings"
)

var (
	reRedirect    = regexp.MustCompile(`(?i)^#REDIRECT\s*\[\[([^\]|]+)`)
	reHeading     = regexp.MustCompile(`^(==+)\s*(.+?)\s*\1\s*$`)
	reRefOpen     = regexp.MustCompile(`(?s)<ref[^>]*>.*?</ref>`)
	reRefSelf     = regexp.MustCompile(`<ref[^>]*/\s*>`)
	reBrTag       = regexp.MustCompile(`<br\s*/?>`)
	reHTMLTag     = regexp.MustCompile(`<[a-zA-Z/][^>]*>`)
	reExternalLink = regexp.MustCompile(`\[https?://\S+\s+[^\]]+\]`)
	reExternalLinkNoText = regexp.MustCompile(`\[https?://\S+\]`)
)

func ParsePage(title, wikitext string) *ParsedPage {
	p := &ParsedPage{Title: title}

	// Rule 1: redirect
	for l := range strings.SplitSeq(wikitext, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if m := reRedirect.FindStringSubmatch(trimmed); m != nil {
			p.IsRedirect = strings.TrimSpace(m[1])
			return p
		}
		break
	}

	// Rule 2: disambiguation
	lower := strings.ToLower(wikitext)
	if strings.Contains(lower, "{{disambiguation") || strings.Contains(lower, "{{disambig") {
		p.IsDisambig = true
	}

	// Strip then parse sections
	cleaned := preprocess(wikitext)

	var leadLines []string
	var currentTitle string
	var currentLevel int
	var currentLines []string
	inLead := true

	flushSection := func() {
		if !inLead && currentTitle != "" {
			body := strings.TrimSpace(strings.Join(currentLines, "\n"))
			idx := len(p.Sections)
			p.Sections = append(p.Sections, Section{
				Title: currentTitle,
				Level: currentLevel,
				Body:  body,
			})
			if currentLevel == 2 || currentLevel == 3 {
				p.Outline = append(p.Outline, OutlineItem{
					Title: currentTitle,
					Level: currentLevel,
					Index: idx,
				})
			}
		}
	}

	for line := range strings.SplitSeq(cleaned, "\n") {
		if m := reHeading.FindStringSubmatch(line); m != nil {
			level := len(m[1]) / 2
			heading := m[2]

			if inLead {
				flushSection()
				inLead = false
			} else {
				flushSection()
			}

			currentTitle = heading
			currentLevel = level
			currentLines = nil

			// emit markdown heading
			prefix := strings.Repeat("#", level+1)
			currentLines = append(currentLines, prefix+" "+heading)
			continue
		}

		if inLead {
			leadLines = append(leadLines, line)
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flushSection()

	p.Lead = strings.TrimSpace(strings.Join(leadLines, "\n"))
	return p
}

func ExtractLead(wikitext string) string {
	lines := strings.Split(wikitext, "\n")
	var result []string
	for _, line := range lines {
		if reHeading.MatchString(line) {
			break
		}
		result = append(result, convertInline(line))
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func preprocess(wikitext string) string {
	s := wikitext

	// Rule 4: strip ref tags
	s = reRefOpen.ReplaceAllString(s, "")
	s = reRefSelf.ReplaceAllString(s, "")

	// Rule 3: strip infoboxes
	s = stripTemplateByPrefix(s, "infobox")

	// Rule 5: strip cite/reflist templates
	s = stripTemplateByPrefix(s, "cite ")
	s = stripTemplateByPrefix(s, "cite:")
	s = stripExactTemplate(s, "reflist")

	// Rule 10: strip tables
	s = stripTables(s)

	var out strings.Builder
	for line := range strings.SplitSeq(s, "\n") {
		// Rule 12: skip category/file/image lines
		lower := strings.ToLower(line)
		trimmed := strings.TrimSpace(lower)
		if strings.HasPrefix(trimmed, "[[category:") ||
			strings.HasPrefix(trimmed, "[[file:") ||
			strings.HasPrefix(trimmed, "[[image:") {
			continue
		}
		out.WriteString(convertInline(line))
		out.WriteByte('\n')
	}
	return out.String()
}

func convertInline(line string) string {
	// Rule 11: br tags
	line = reBrTag.ReplaceAllString(line, "")
	// Rule 11: other html tags
	line = reHTMLTag.ReplaceAllString(line, "")
	// Rule 9: external links with text
	line = reExternalLink.ReplaceAllStringFunc(line, func(s string) string {
		inner := s[1 : len(s)-1]
		url, text, found := strings.Cut(inner, " ")
		if !found {
			return s
		}
		return "[" + strings.TrimSpace(text) + "](" + url + ")"
	})
	// Rule 9: external links without text
	line = reExternalLinkNoText.ReplaceAllString(line, "")
	// Rule 8: wikilinks [[Article|display]]
	line = convertWikilinks(line)
	// Rule 7: bold/italic
	line = strings.ReplaceAll(line, "'''", "**")
	line = strings.ReplaceAll(line, "''", "*")
	return line
}

func convertWikilinks(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '[' && s[i+1] == '[' {
			end := strings.Index(s[i:], "]]")
			if end < 0 {
				b.WriteByte(s[i])
				i++
				continue
			}
			inner := s[i+2 : i+end]
			article, display, found := strings.Cut(inner, "|")
			if !found {
				display = article
			}
			b.WriteString("[" + display + "](" + article + ")")
			i += end + 2
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// stripTemplateByPrefix strips {{prefix ...}} templates by counting nesting.
func stripTemplateByPrefix(s, prefix string) string {
	lowerS := strings.ToLower(s)
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			// check if this template starts with the prefix
			inner := lowerS[i+2:]
			if strings.HasPrefix(inner, prefix) {
				// skip entire template including nested braces
				depth := 1
				j := i + 2
				for j+1 < len(s) && depth > 0 {
					if s[j] == '{' && s[j+1] == '{' {
						depth++
						j += 2
					} else if s[j] == '}' && s[j+1] == '}' {
						depth--
						j += 2
					} else {
						j++
					}
				}
				i = j
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripExactTemplate(s, name string) string {
	lowerS := strings.ToLower(s)
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			inner := strings.TrimSpace(lowerS[i+2:])
			if strings.HasPrefix(inner, name) {
				nextCh := ' '
				if len(name) < len(inner) {
					nextCh = rune(inner[len(name)])
				}
				if nextCh == '}' || nextCh == ' ' || nextCh == '|' || nextCh == '\n' {
					depth := 1
					j := i + 2
					for j+1 < len(s) && depth > 0 {
						if s[j] == '{' && s[j+1] == '{' {
							depth++
							j += 2
						} else if s[j] == '}' && s[j+1] == '}' {
							depth--
							j += 2
						} else {
							j++
						}
					}
					i = j
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripTables(s string) string {
	var b strings.Builder
	lines := strings.Split(s, "\n")
	depth := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{|") {
			depth++
			continue
		}
		if depth > 0 {
			if strings.HasPrefix(trimmed, "{|") {
				depth++
			} else if trimmed == "|}" {
				depth--
			}
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
