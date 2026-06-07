package rest

import (
	"encoding/json"
	"testing"
)

// TestADFBuilder_PlainText verifies that a plain-text paragraph produces the
// exact ADF v3 JSON structure required by Jira (REQ-COMMENT, golden fixture).
func TestADFBuilder_PlainText(t *testing.T) {
	b := NewADFBuilder()
	b.AddParagraph("Hello, world!")
	doc := b.Build()

	got, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	want := `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hello, world!"}]}]}`
	if string(got) != want {
		t.Errorf("plain text ADF mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

// TestADFBuilder_TextWithLink verifies that a link mark produces the exact
// JSON shape Jira expects: text node with a "link" mark containing href.
func TestADFBuilder_TextWithLink(t *testing.T) {
	b := NewADFBuilder()
	b.AddParagraphWithLink("See PR", "https://github.com/org/repo/pull/42")
	doc := b.Build()

	got, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	want := `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"See PR","marks":[{"type":"link","attrs":{"href":"https://github.com/org/repo/pull/42"}}]}]}]}`
	if string(got) != want {
		t.Errorf("link ADF mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

// TestADFBuilder_MultiParagraph verifies that multiple AddParagraph calls
// produce multiple paragraph nodes in the doc content array.
func TestADFBuilder_MultiParagraph(t *testing.T) {
	b := NewADFBuilder()
	b.AddParagraph("First paragraph.")
	b.AddParagraph("Second paragraph.")
	doc := b.Build()

	if len(doc.Content) != 2 {
		t.Errorf("want 2 paragraphs, got %d", len(doc.Content))
	}
	if doc.Content[0].Content[0].Text != "First paragraph." {
		t.Errorf("para[0] text = %q, want %q", doc.Content[0].Content[0].Text, "First paragraph.")
	}
	if doc.Content[1].Content[0].Text != "Second paragraph." {
		t.Errorf("para[1] text = %q, want %q", doc.Content[1].Content[0].Text, "Second paragraph.")
	}
}

// TestADFBuilder_EmptyDoc verifies a Build() with no content still yields a
// valid ADF doc envelope (version:1, type:doc, empty content array).
func TestADFBuilder_EmptyDoc(t *testing.T) {
	b := NewADFBuilder()
	doc := b.Build()

	if doc.Version != 1 {
		t.Errorf("version = %d, want 1", doc.Version)
	}
	if doc.Type != "doc" {
		t.Errorf("type = %q, want %q", doc.Type, "doc")
	}

	got, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"version":1,"type":"doc","content":[]}`
	if string(got) != want {
		t.Errorf("empty doc mismatch\ngot:  %s\nwant: %s", got, want)
	}
}
