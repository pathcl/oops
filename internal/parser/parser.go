package parser

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Section represents one cheatsheet entry.
type Section struct {
	Category  string // H2 heading (e.g. "PromQL")
	Title     string // H3 heading (e.g. "Rate of requests")
	Body      string // prose description text
	CodeBlock string // the query itself
	Lang      string // code fence language hint
}

// Parse extracts sections from a markdown document.
// Structure: ## Category > ### Title > prose + fenced code block.
func Parse(src string) []Section {
	source := []byte(src)
	reader := text.NewReader(source)

	md := goldmark.New()
	doc := md.Parser().Parse(reader)

	var sections []Section
	var currentCategory string
	var current *Section

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			text := headingText(node, source)
			switch node.Level {
			case 2:
				if current != nil {
					sections = append(sections, *current)
					current = nil
				}
				currentCategory = text
			case 3:
				if current != nil {
					sections = append(sections, *current)
				}
				current = &Section{
					Category: currentCategory,
					Title:    text,
				}
			}

		case *ast.FencedCodeBlock:
			if current == nil {
				return ast.WalkContinue, nil
			}
			lang := string(node.Language(source))
			var buf bytes.Buffer
			for i := 0; i < node.Lines().Len(); i++ {
				line := node.Lines().At(i)
				buf.Write(line.Value(source))
			}
			current.Lang = lang
			current.CodeBlock = strings.TrimRight(buf.String(), "\n")

		case *ast.Paragraph:
			if current == nil {
				return ast.WalkContinue, nil
			}
			var buf bytes.Buffer
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if t, ok := child.(*ast.Text); ok {
					buf.Write(t.Segment.Value(source))
					if t.SoftLineBreak() || t.HardLineBreak() {
						buf.WriteByte('\n')
					}
				}
			}
			text := strings.TrimSpace(buf.String())
			if text != "" {
				if current.Body != "" {
					current.Body += "\n" + text
				} else {
					current.Body = text
				}
			}
		}

		return ast.WalkContinue, nil
	})

	if current != nil {
		sections = append(sections, *current)
	}

	return sections
}

func headingText(n *ast.Heading, source []byte) string {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		}
	}
	return strings.TrimSpace(buf.String())
}
