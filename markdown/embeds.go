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
	"github.com/yuin/goldmark/renderer"
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

	for m, v := range mimePrefixes {
		if strings.HasPrefix(mimeType, m) {
			block.Advance(endIndex + 2)
			element := newMediaEmbed(v, fmt.Sprintf("/files/view/%s", target))
			return element
		}
	}

	return nil
}

type mediaType int

const (
	image mediaType = iota
	video
	audio
)

var mimePrefixes = map[string]mediaType{
	"image/": image,
	"video/": video,
	"audio/": audio,
}

type mediaEmbed struct {
	mediaType mediaType
	file      string
	ast.BaseBlock
}

var kindMediaEmbed = ast.NewNodeKind("MediaEmbed")

func newMediaEmbed(m mediaType, file string) *mediaEmbed {
	return &mediaEmbed{
		mediaType: m,
		file:      file,
	}
}

func (m *mediaEmbed) Dump(source []byte, level int) {
	ast.DumpHelper(m, source, level, map[string]string{}, nil)
}

func (m *mediaEmbed) Kind() ast.NodeKind {
	return kindMediaEmbed
}

func (m *mediaEmbed) IsRaw() bool {
	return true
}

type mediaRenderer struct{}

func newMediaRenderer() renderer.NodeRenderer {
	return &mediaRenderer{}
}

func (m mediaRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMediaEmbed, m.render)
}

func (m mediaRenderer) render(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		return ast.WalkContinue, nil
	}

	embed := n.(*mediaEmbed)
	switch embed.mediaType {
	case image:
		_, _ = w.WriteString(fmt.Sprintf(`<img src="%s" class="embed">`, embed.file))
	case audio:
		_, _ = w.WriteString(fmt.Sprintf(`<audio controls src="%s" class="embed">`, embed.file))
	case video:
		_, _ = w.WriteString(fmt.Sprintf(`<video controls src="%s" class="embed">`, embed.file))
	}

	return ast.WalkContinue, nil
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
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newMediaRenderer(), 500),
	))
}
