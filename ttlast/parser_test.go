package ttlast

import (
	"os"
	"strings"
	"testing"
)

func TestParser_ParsePrefix(t *testing.T) {
	parser := NewParser()
	input := `
@prefix ex: <http://example.org/> .
@prefix skos: <http://www.w3.org/2004/02/skos/core#> .
`
	doc, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(doc.Prefixes) != 2 {
		t.Errorf("Expected 2 prefixes, got %d", len(doc.Prefixes))
	}

	if doc.Prefixes[0].Prefix != "ex" {
		t.Errorf("Expected prefix 'ex', got '%s'", doc.Prefixes[0].Prefix)
	}

	if doc.Prefixes[0].IRI.Value != "http://example.org/" {
		t.Errorf("Expected IRI 'http://example.org/', got '%s'", doc.Prefixes[0].IRI.Value)
	}
}

func TestParser_ParseTriple(t *testing.T) {
	parser := NewParser()
	input := `
@prefix ex: <http://example.org/> .
@prefix skos: <http://www.w3.org/2004/02/skos/core#> .

ex:course a skos:Concept .
`
	doc, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(doc.Statements) != 1 {
		t.Errorf("Expected 1 statement, got %d", len(doc.Statements))
	}

	stmt := doc.Statements[0]
	if stmt.Triple.Subject == nil {
		t.Error("Expected subject to be non-nil")
	}
}

func TestParser_ParseLiteral(t *testing.T) {
	parser := NewParser()
	input := `
@prefix ex: <http://example.org/> .
@prefix skos: <http://www.w3.org/2004/02/skos/core#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:course skos:prefLabel "Course"@en .
ex:count skos:notation "123"^^xsd:integer .
ex:flag skos:example "true"^^xsd:boolean .
`
	doc, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(doc.Statements) != 3 {
		t.Errorf("Expected 3 statements, got %d", len(doc.Statements))
	}
}

func TestParser_ParseCollection(t *testing.T) {
	parser := NewParser()
	input := `
@prefix ex: <http://example.org/> .
@prefix skos: <http://www.w3.org/2004/02/skos/core#> .

ex:list skos:memberList ( ex:a ex:b ex:c ) .
`
	doc, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(doc.Statements) != 1 {
		t.Errorf("Expected 1 statement, got %d", len(doc.Statements))
	}
}

func TestParser_PositionTracking(t *testing.T) {
	parser := NewParser()
	input := `
@prefix ex: <http://example.org/> .
ex:course a <http://www.w3.org/2004/02/skos/core#Concept> .
`
	doc, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(doc.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(doc.Statements))
	}

	stmt := doc.Statements[0]
	if stmt.Pos() <= 0 {
		t.Error("Expected positive position")
	}

	if stmt.End() <= stmt.Pos() {
		t.Error("Expected end > start position")
	}
}

func TestParser_ParseFile(t *testing.T) {
	parser := NewParser()
	doc, err := parser.ParseFile("../testdata/skos.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	if len(doc.Prefixes) == 0 {
		t.Error("Expected prefixes in document")
	}

	if len(doc.Statements) == 0 {
		t.Error("Expected statements in document")
	}
}

func TestWalk(t *testing.T) {
	parser := NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/transitive_broader.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	var count int
	Walk(DefaultVisitor{}, doc)
	count = len(PreOrder(doc))

	if count == 0 {
		t.Error("Expected to visit nodes in pre-order")
	}
}

func BenchmarkParser_Parse(b *testing.B) {
	data, _ := os.ReadFile("../testdata/skos.ttl")
	input := string(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser()
		_, err := parser.Parse(strings.NewReader(input))
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}
