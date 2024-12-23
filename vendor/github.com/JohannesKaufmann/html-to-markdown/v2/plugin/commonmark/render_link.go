package commonmark

import (
	"bytes"
	"strings"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/internal/textutils"
	"github.com/JohannesKaufmann/html-to-markdown/v2/marker"
	"golang.org/x/net/html"
)

// link in commonmark contains
// - the link text (the visible text)
// - a link destination (the URI that is the link destination)
// - an optional link title
type link struct {
	*html.Node

	before  []byte
	content []byte
	after   []byte

	href  string
	title string
}

func (c *commonmark) renderLinkInlined(w converter.Writer, l *link) converter.RenderStatus {

	w.Write(l.before)
	w.WriteRune('[')
	w.Write(l.content)
	w.WriteRune(']')
	w.WriteRune('(')
	w.WriteString(l.href)
	if l.title != "" {
		// The destination and title must be seperated by a space
		w.WriteRune(' ')
		w.Write(textutils.SurroundByQuotes([]byte(l.title)))
	}
	w.WriteRune(')')
	w.Write(l.after)

	return converter.RenderSuccess
}

func (c *commonmark) renderLink(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	ctx = ctx.WithValue("is_inside_link", true)

	href := dom.GetAttributeOr(n, "href", "")

	href = strings.TrimSpace(href)
	href = ctx.AssembleAbsoluteURL(ctx, "a", href)

	title := dom.GetAttributeOr(n, "title", "")
	title = strings.ReplaceAll(title, "\n", " ")

	l := &link{
		Node:  n,
		href:  href,
		title: title,
	}

	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := buf.Bytes()

	if bytes.TrimFunc(content, marker.IsSpace) == nil {
		// Fallback to the title
		content = []byte(l.title)
	}
	if bytes.TrimSpace(content) == nil {
		return converter.RenderSuccess
	}

	if l.href == "" {
		// A link without href is valid, like e.g. [text]()
		// But a title would make it invalid.
		l.title = ""
	}

	leftExtra, trimmed, rightExtra := textutils.SurroundingSpaces(content)

	trimmed = textutils.EscapeMultiLine(trimmed)

	l.before = leftExtra
	l.content = trimmed
	l.after = rightExtra

	switch c.LinkStyle {
	case LinkStyleInlined:
		return c.renderLinkInlined(w, l)
	default:
		return converter.RenderTryNext
	}
}
