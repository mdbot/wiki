package markdown

import (
	"bytes"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type embedParser struct {
	checker PageChecker
}

func newEmbedParser() parser.InlineParser {
	return &embedParser{}
}

func (w *embedParser) Trigger() []byte {
	return []byte{'!'}
}

func (w *embedParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()

	if len(line) < 2 || line[1] != '[' || line[2] != '[' {
		return nil
	}

	endIndex := bytes.Index(line, []byte{']', ']'})
	if endIndex == -1 {
		return nil
	}

	target := line[3:endIndex]
	mimeType := mime.TypeByExtension(filepath.Ext(string(target)))
	if strings.HasPrefix(mimeType, "image/") {
		block.Advance(endIndex + 2)
		link := ast.NewLink()
		link.Title = target
		link.Destination = []byte(fmt.Sprintf("/file/%s", target))
		return ast.NewImage(link)
	}

	return nil
}

type embedExtension struct {
}

func newEmbedExtension() goldmark.Extender {
	return &embedExtension{}
}

func (e *embedExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(newEmbedParser(), 101),
	))
}
