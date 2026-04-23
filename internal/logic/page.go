package logic

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/tyrm/mcp-wikipedia-local/internal/parser"
)

type GetPageResponse struct {
	Title         string
	Content       string
	Outline       []parser.OutlineItem
	RedirectChain []string
	IsDisambig    bool
}

func (l *Logic) GetPage(ctx context.Context, title, mode, section string) (*GetPageResponse, error) {
	return l.getPageWithChain(ctx, title, mode, section, nil)
}

func (l *Logic) getPageWithChain(ctx context.Context, title, mode, section string, chain []string) (*GetPageResponse, error) {
	if slices.Contains(chain, title) {
		return nil, fmt.Errorf("redirect loop detected: %s", title)
	}
	if len(chain) > 3 {
		return nil, fmt.Errorf("too many redirects")
	}

	resolved, err := l.resolveTitle(ctx, title)
	if err != nil {
		return nil, err
	}

	if page, ok := l.cfg.Cache.Get(resolved); ok {
		return l.paginatePage(page, mode, section, chain), nil
	}

	wikitext, err := l.cfg.Archive.GetPage(resolved)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	page := parser.ParsePage(resolved, wikitext)
	l.cfg.Cache.Set(resolved, page)

	if page.IsRedirect != "" {
		return l.getPageWithChain(ctx, page.IsRedirect, mode, section, append(chain, resolved))
	}

	return l.paginatePage(page, mode, section, chain), nil
}

func (l *Logic) paginatePage(page *parser.ParsedPage, mode, section string, chain []string) *GetPageResponse {
	resp := &GetPageResponse{
		Title:         page.Title,
		Outline:       page.Outline,
		IsDisambig:    page.IsDisambig,
		RedirectChain: chain,
	}

	switch mode {
	case "lead":
		resp.Content = page.Lead
	case "outline":
		var sb strings.Builder
		for _, item := range page.Outline {
			sb.WriteString(strings.Repeat("#", item.Level) + " " + item.Title + "\n")
		}
		resp.Content = sb.String()
	case "section":
		for _, s := range page.Sections {
			if strings.EqualFold(s.Title, section) {
				resp.Content = s.Body
				return resp
			}
		}
		resp.Content = ""
	default:
		var parts []string
		if page.Lead != "" {
			parts = append(parts, page.Lead)
		}
		for _, s := range page.Sections {
			parts = append(parts, s.Body)
		}
		resp.Content = strings.Join(parts, "\n\n")
	}

	return resp
}
