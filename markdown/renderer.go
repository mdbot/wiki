package markdown

import (
	"bytes"
	"net/url"

	wikilink "github.com/13rac1/goldmark-wikilink"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

type PageChecker interface {
	PageExists(name string) bool
}

type Renderer struct {
	checker PageChecker
	gm      goldmark.Markdown
}

func NewRenderer(checker PageChecker, codeStyle string) *Renderer {
	return &Renderer{
		checker: checker,
		gm: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				highlighting.NewHighlighting(highlighting.WithStyle(codeStyle)),
				wikilink.New(wikilink.WithFilenameNormalizer(FileNameNormalizer{})),
			),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		),
	}
}

func (r *Renderer) Render(markdown []byte) (string, error) {
	b := &bytes.Buffer{}
	if err := r.gm.Convert(markdown, b); err != nil {
		return "", err
	}
	return b.String(), nil
}

type FileNameNormalizer struct{}

func (_ FileNameNormalizer) Normalize(linkText string) string {
	return url.PathEscape(linkText)
}
