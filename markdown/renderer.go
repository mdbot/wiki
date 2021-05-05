package markdown

import (
	"bytes"

	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/mdigger/goldmark-attributes"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type PageChecker interface {
	PageExists(name string) bool
}

type Renderer struct {
	checker    PageChecker
	gm         goldmark.Markdown
	htmlPolicy *bluemonday.Policy
}

func NewRenderer(checker PageChecker, dangerousHtml bool, codeStyle string) *Renderer {
	var htmlPolicy *bluemonday.Policy
	if !dangerousHtml {
		htmlPolicy = bluemonday.UGCPolicy()
	}

	return &Renderer{
		checker:    checker,
		htmlPolicy: htmlPolicy,
		gm: goldmark.New(
			goldmark.WithExtensions(
				mathjax.MathJax,
				extension.GFM,
				highlighting.NewHighlighting(highlighting.WithStyle(codeStyle)),
				newWikiLinks(checker),
				newEmbedExtension(),
				attributes.Extension,
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		),
	}
}

func (r *Renderer) Render(markdown []byte) (string, error) {
	b := &bytes.Buffer{}
	if err := r.gm.Convert(markdown, b); err != nil {
		return "", err
	}
	if r.htmlPolicy != nil {
		return r.htmlPolicy.Sanitize(b.String()), nil
	}
	return b.String(), nil
}
