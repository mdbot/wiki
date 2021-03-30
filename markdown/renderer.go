package markdown

import (
	"bytes"

	mathjax "github.com/litao91/goldmark-mathjax"
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
				mathjax.MathJax,
				extension.GFM,
				highlighting.NewHighlighting(highlighting.WithStyle(codeStyle)),
				newWikiLinks(checker),
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
