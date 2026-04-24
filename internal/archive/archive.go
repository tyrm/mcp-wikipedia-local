package archive

import (
	"bufio"
	"compress/bzip2"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Archive struct {
	cfg     *Config
	index   map[string]int64
	offsets []int64
	file    *os.File
	mu      sync.RWMutex
}

func New(cfg *Config) (*Archive, error) {
	f, err := os.Open(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	return &Archive{
		cfg:   cfg,
		index: make(map[string]int64),
		file:  f,
	}, nil
}

func (a *Archive) LoadIndex() error {
	f, err := os.Open(a.cfg.IndexPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer f.Close()

	r := bzip2.NewReader(f)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	offsetSet := make(map[int64]struct{})

	for scanner.Scan() {
		line := scanner.Text()
		first := strings.Index(line, ":")
		if first < 0 {
			continue
		}
		second := strings.Index(line[first+1:], ":")
		if second < 0 {
			continue
		}

		offsetStr := line[:first]
		title := html.UnescapeString(line[first+1+second+1:])

		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			continue
		}

		if !isArticleTitle(title) {
			continue
		}
		a.index[title] = offset
		offsetSet[offset] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan index: %w", err)
	}

	a.offsets = make([]int64, 0, len(offsetSet))
	for o := range offsetSet {
		a.offsets = append(a.offsets, o)
	}
	slices.Sort(a.offsets)

	return nil
}

func (a *Archive) GetPage(title string) (string, error) {
	a.mu.RLock()
	offset, ok := a.index[title]
	a.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("title not found: %s", title)
	}

	streamSize := a.nextOffset(offset) - offset
	if streamSize <= 0 {
		streamSize = 10 * 1024 * 1024
	}

	sr := io.NewSectionReader(a.file, offset, streamSize)
	bzr := bzip2.NewReader(sr)

	return extractPage(bzr, title)
}

func (a *Archive) nextOffset(offset int64) int64 {
	a.mu.RLock()
	offsets := a.offsets
	a.mu.RUnlock()

	i := sort.Search(len(offsets), func(i int) bool { return offsets[i] > offset })
	if i < len(offsets) {
		return offsets[i]
	}
	return offset + 10*1024*1024
}

func (a *Archive) Titles() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	titles := make([]string, 0, len(a.index))
	for t := range a.index {
		titles = append(titles, t)
	}
	return titles
}

func (a *Archive) OffsetForTitle(title string) (int64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	o, ok := a.index[title]
	return o, ok
}

func (a *Archive) Close() error {
	return a.file.Close()
}

// wikipediaNamespaces lists all non-article namespace prefixes in English Wikipedia.
var wikipediaNamespaces = map[string]struct{}{
	"talk": {}, "user": {}, "user talk": {}, "wikipedia": {}, "wikipedia talk": {},
	"file": {}, "file talk": {}, "mediawiki": {}, "mediawiki talk": {},
	"template": {}, "template talk": {}, "help": {}, "help talk": {},
	"category": {}, "category talk": {}, "portal": {}, "portal talk": {},
	"book": {}, "book talk": {}, "draft": {}, "draft talk": {},
	"timedtext": {}, "timedtext talk": {}, "module": {}, "module talk": {},
	"gadget": {}, "gadget talk": {}, "gadget definition": {}, "gadget definition talk": {},
	"education program": {}, "education program talk": {},
	"special": {}, "media": {},
}

// isArticleTitle returns true if title belongs to the main (NS=0) namespace.
func isArticleTitle(title string) bool {
	if i := strings.Index(title, ":"); i > 0 {
		prefix := strings.ToLower(title[:i])
		if _, ok := wikipediaNamespaces[prefix]; ok {
			return false
		}
	}
	return true
}

type xmlPage struct {
	Title    string `xml:"title"`
	NS       int    `xml:"ns"`
	Revision struct {
		Text string `xml:"text"`
	} `xml:"revision"`
}

func extractPage(r io.Reader, title string) (string, error) {
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "page" {
			continue
		}
		var p xmlPage
		if err := dec.DecodeElement(&p, &se); err != nil {
			continue
		}
		if p.NS != 0 {
			continue
		}
		if p.Title == title {
			return p.Revision.Text, nil
		}
	}
	return "", fmt.Errorf("page not found in stream: %s", title)
}
