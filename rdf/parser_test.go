package rdf

import (
	"os"
	"strings"
	"testing"

	"github.com/soypete/ontology-go/types"
)

func TestXMLParser_BasicDescription(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <rdf:Description rdf:about="http://example.org/book1">
    <dc:title>A Book</dc:title>
    <dc:creator>An Author</dc:creator>
  </rdf:Description>
</rdf:RDF>`

	parser := NewXMLParser("test")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}

	assertTriple(t, triples[0], "http://example.org/book1", "http://purl.org/dc/elements/1.1/title", "A Book")
	assertTriple(t, triples[1], "http://example.org/book1", "http://purl.org/dc/elements/1.1/creator", "An Author")

	// Check graph assignment
	for _, tr := range triples {
		if tr.Graph != "test" {
			t.Errorf("expected graph 'test', got %q", tr.Graph)
		}
	}
}

func TestXMLParser_TypedNode(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:foaf="http://xmlns.com/foaf/0.1/">
  <foaf:Person rdf:about="http://example.org/alice">
    <foaf:name>Alice</foaf:name>
  </foaf:Person>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}

	// First triple: rdf:type
	assertTriple(t, triples[0], "http://example.org/alice",
		"http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
		"http://xmlns.com/foaf/0.1/Person")

	// Second triple: foaf:name
	assertTriple(t, triples[1], "http://example.org/alice",
		"http://xmlns.com/foaf/0.1/name", "Alice")
}

func TestXMLParser_ResourceAttribute(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:foaf="http://xmlns.com/foaf/0.1/">
  <rdf:Description rdf:about="http://example.org/alice">
    <foaf:homepage rdf:resource="http://example.org/alice-homepage"/>
  </rdf:Description>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}

	assertTriple(t, triples[0], "http://example.org/alice",
		"http://xmlns.com/foaf/0.1/homepage",
		"http://example.org/alice-homepage")
}

func TestXMLParser_NestedDescription(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:foaf="http://xmlns.com/foaf/0.1/">
  <foaf:Person rdf:about="http://example.org/alice">
    <foaf:knows>
      <foaf:Person rdf:about="http://example.org/bob">
        <foaf:name>Bob</foaf:name>
      </foaf:Person>
    </foaf:knows>
  </foaf:Person>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected triples:
	// 1. alice rdf:type foaf:Person
	// 2. alice foaf:knows bob
	// 3. bob rdf:type foaf:Person
	// 4. bob foaf:name "Bob"
	if len(triples) != 4 {
		t.Fatalf("expected 4 triples, got %d: %+v", len(triples), triples)
	}

	assertTriple(t, triples[0], "http://example.org/alice",
		"http://www.w3.org/1999/02/22-rdf-syntax-ns#type",
		"http://xmlns.com/foaf/0.1/Person")

	// Find the knows triple
	found := false
	for _, tr := range triples {
		if tr.Predicate == "http://xmlns.com/foaf/0.1/knows" {
			if tr.Subject != "http://example.org/alice" || tr.Object != "http://example.org/bob" {
				t.Errorf("unexpected knows triple: %+v", tr)
			}
			found = true
		}
	}
	if !found {
		t.Error("missing foaf:knows triple")
	}
}

func TestXMLParser_BlankNode(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:foaf="http://xmlns.com/foaf/0.1/">
  <foaf:Person>
    <foaf:name>Anonymous</foaf:name>
  </foaf:Person>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}

	// Subject should be a blank node
	if !strings.HasPrefix(triples[0].Subject, "_:") {
		t.Errorf("expected blank node subject, got %q", triples[0].Subject)
	}

	// Both triples should have the same blank node subject
	if triples[0].Subject != triples[1].Subject {
		t.Errorf("blank node subjects differ: %q vs %q", triples[0].Subject, triples[1].Subject)
	}
}

func TestXMLParser_Datatype(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:foaf="http://xmlns.com/foaf/0.1/">
  <foaf:Person rdf:about="http://example.org/alice">
    <foaf:age rdf:datatype="http://www.w3.org/2001/XMLSchema#integer">30</foaf:age>
  </foaf:Person>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have rdf:type and age
	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}

	assertTriple(t, triples[1], "http://example.org/alice",
		"http://xmlns.com/foaf/0.1/age", "30")
}

func TestXMLParser_MultipleNamespaces(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:rdfs="http://www.w3.org/2000/01/rdf-schema#"
         xmlns:dc="http://purl.org/dc/elements/1.1/"
         xmlns:foaf="http://xmlns.com/foaf/0.1/">
  <rdf:Description rdf:about="http://example.org/resource1">
    <dc:title>Resource One</dc:title>
    <rdfs:label>R1</rdfs:label>
    <foaf:homepage rdf:resource="http://example.org/r1"/>
  </rdf:Description>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 3 {
		t.Fatalf("expected 3 triples, got %d", len(triples))
	}

	assertTriple(t, triples[0], "http://example.org/resource1",
		"http://purl.org/dc/elements/1.1/title", "Resource One")
	assertTriple(t, triples[1], "http://example.org/resource1",
		"http://www.w3.org/2000/01/rdf-schema#label", "R1")
	assertTriple(t, triples[2], "http://example.org/resource1",
		"http://xmlns.com/foaf/0.1/homepage", "http://example.org/r1")
}

func TestXMLParser_FOAFFixture(t *testing.T) {
	f, err := os.Open("../testdata/foaf.rdf")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	parser := NewXMLParser("foaf")
	triples, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse foaf fixture: %v", err)
	}

	if len(triples) == 0 {
		t.Fatal("expected triples from FOAF fixture, got none")
	}

	// Verify we got Alice
	found := false
	for _, tr := range triples {
		if tr.Subject == "http://example.org/people/alice" && tr.Predicate == "http://xmlns.com/foaf/0.1/name" {
			if tr.Object != "Alice Smith" {
				t.Errorf("expected Alice Smith, got %q", tr.Object)
			}
			found = true
		}
	}
	if !found {
		t.Error("missing Alice Smith triple from FOAF fixture")
	}

	// Verify graph is set
	for _, tr := range triples {
		if tr.Graph != "foaf" {
			t.Errorf("expected graph 'foaf', got %q", tr.Graph)
			break
		}
	}
}

func TestXMLParser_WikidataFixture(t *testing.T) {
	f, err := os.Open("../testdata/wikidata_sample.rdf")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	parser := NewXMLParser("wikidata")
	triples, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse wikidata fixture: %v", err)
	}

	if len(triples) == 0 {
		t.Fatal("expected triples from Wikidata fixture, got none")
	}

	// Douglas Adams (Q42) should have label "Douglas Adams"
	found := false
	for _, tr := range triples {
		if tr.Subject == "http://www.wikidata.org/entity/Q42" &&
			tr.Predicate == "http://www.w3.org/2000/01/rdf-schema#label" &&
			tr.Object == "Douglas Adams" {
			found = true
		}
	}
	if !found {
		t.Error("missing Douglas Adams rdfs:label triple from Wikidata fixture")
	}

	// Q42 should be instance of Q5 (human)
	found = false
	for _, tr := range triples {
		if tr.Subject == "http://www.wikidata.org/entity/Q42" &&
			tr.Predicate == "http://www.wikidata.org/prop/direct/P31" &&
			tr.Object == "http://www.wikidata.org/entity/Q5" {
			found = true
		}
	}
	if !found {
		t.Error("missing Q42 P31 Q5 triple from Wikidata fixture")
	}
}

func TestXMLParser_MalformedInput(t *testing.T) {
	parser := NewXMLParser("")

	_, err := parser.ParseString("not xml at all <<<>>>")
	if err == nil {
		t.Error("expected error for malformed input, got nil")
	}
}

func TestXMLParser_EmptyRDF(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 0 {
		t.Errorf("expected 0 triples, got %d", len(triples))
	}
}

func TestXMLParser_MultipleRDFResourceOnSameProperty(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:wdt="http://www.wikidata.org/prop/direct/">
  <rdf:Description rdf:about="http://example.org/person">
    <wdt:P106 rdf:resource="http://example.org/writer"/>
    <wdt:P106 rdf:resource="http://example.org/actor"/>
  </rdf:Description>
</rdf:RDF>`

	parser := NewXMLParser("")
	triples, err := parser.ParseString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}

	assertTriple(t, triples[0], "http://example.org/person",
		"http://www.wikidata.org/prop/direct/P106", "http://example.org/writer")
	assertTriple(t, triples[1], "http://example.org/person",
		"http://www.wikidata.org/prop/direct/P106", "http://example.org/actor")
}

func assertTriple(t *testing.T, got types.Triple, wantSubject, wantPredicate, wantObject string) {
	t.Helper()
	if got.Subject != wantSubject {
		t.Errorf("subject: got %q, want %q", got.Subject, wantSubject)
	}
	if got.Predicate != wantPredicate {
		t.Errorf("predicate: got %q, want %q", got.Predicate, wantPredicate)
	}
	if got.Object != wantObject {
		t.Errorf("object: got %q, want %q", got.Object, wantObject)
	}
}
