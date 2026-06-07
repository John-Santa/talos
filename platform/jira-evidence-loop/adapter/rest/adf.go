package rest

import (
	"github.com/John-Santa/talos/platform/jira-evidence-loop/domain/evidence"
)

// ADFBuilder is the interface for constructing ADF v3 documents incrementally.
// It is kept behind an interface so the Fase-1 node set (paragraph/text/link)
// can grow without touching call sites (Design §ADF builder open Q#2).
type ADFBuilder interface {
	// AddParagraph appends a plain-text paragraph to the document.
	AddParagraph(text string)
	// AddParagraphWithLink appends a paragraph containing a single text node
	// decorated with a "link" mark pointing to href.
	AddParagraphWithLink(text, href string)
	// Build returns the completed ADFDocument value.
	Build() evidence.ADFDocument
}

// adfBuilder is the concrete Fase-1 implementation of ADFBuilder.
type adfBuilder struct {
	paragraphs []evidence.ADFNode
}

// NewADFBuilder returns a fresh ADFBuilder ready to accept content.
func NewADFBuilder() ADFBuilder {
	return &adfBuilder{}
}

// AddParagraph appends a paragraph containing a single plain text node.
func (b *adfBuilder) AddParagraph(text string) {
	b.paragraphs = append(b.paragraphs, evidence.ADFNode{
		Type: "paragraph",
		Content: []evidence.ADFNode{
			{Type: "text", Text: text},
		},
	})
}

// AddParagraphWithLink appends a paragraph containing a text node with a
// "link" mark (href attribute). This satisfies REQ-COMMENT minimum node set:
// doc/paragraph/text/link.
func (b *adfBuilder) AddParagraphWithLink(text, href string) {
	b.paragraphs = append(b.paragraphs, evidence.ADFNode{
		Type: "paragraph",
		Content: []evidence.ADFNode{
			{
				Type: "text",
				Text: text,
				Marks: []evidence.ADFMark{
					{
						Type: "link",
						Attrs: map[string]any{
							"href": href,
						},
					},
				},
			},
		},
	})
}

// Build assembles and returns the final ADFDocument. The paragraphs slice is
// never nil in the output — an empty builder yields an empty content array.
func (b *adfBuilder) Build() evidence.ADFDocument {
	content := b.paragraphs
	if content == nil {
		content = []evidence.ADFNode{}
	}
	return evidence.ADFDocument{
		Version: 1,
		Type:    "doc",
		Content: content,
	}
}
