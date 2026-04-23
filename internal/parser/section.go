package parser

type ParsedPage struct {
	Title      string
	Lead       string
	Sections   []Section
	Outline    []OutlineItem
	IsDisambig bool
	IsRedirect string
}

type Section struct {
	Title string
	Level int
	Body  string
}

type OutlineItem struct {
	Title string
	Level int
	Index int
}
