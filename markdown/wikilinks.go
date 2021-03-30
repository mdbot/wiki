package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type wikiLinkParser struct {
	checker PageChecker
}

func newWikiLinkParser(checker PageChecker) parser.InlineParser {
	return &wikiLinkParser{
		checker: checker,
	}
}

func (w *wikiLinkParser) Trigger() []byte {
	return []byte{'['}
}

func (w *wikiLinkParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, segment := block.PeekLine()

	if len(line) == 0 || line[1] != '[' {
		return nil
	}

	endIndex := bytes.Index(line, []byte{']', ']'})
	if endIndex == -1 {
		return nil
	}

	pipeIndex := bytes.Index(line[:endIndex], []byte{'|'})

	block.Advance(endIndex + 2)

	var target []byte
	if pipeIndex == -1 {
		target = line[2:endIndex]
	} else {
		target = line[2:pipeIndex]
	}

	link := ast.NewLink()
	link.Title = target
	link.Destination = target
	if w.checker.PageExists(string(target)) {
		link.SetAttributeString("class", []byte("wikilink"))
	} else {
		link.SetAttributeString("class", []byte("wikilink newpage"))
	}

	t := ast.NewText()
	if pipeIndex == -1 {
		t.Segment = text.NewSegment(segment.Start+2, segment.Start+endIndex)
	} else {
		t.Segment = text.NewSegment(segment.Start+pipeIndex+1, segment.Start+endIndex)
	}
	link.AppendChild(link, t)

	return link
}

type wikiLinkExtension struct {
	checker PageChecker
}

func newWikiLinks(checker PageChecker) goldmark.Extender {
	return &wikiLinkExtension{checker: checker}
}

func (e *wikiLinkExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(newWikiLinkParser(e.checker), 102),
	))
}
