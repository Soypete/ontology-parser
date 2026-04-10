package ttl

import (
	"strings"
	"testing"

	"github.com/soypete/ontology-go/rdf"
	"github.com/soypete/ontology-go/types"
)

func TestTurtleParser_PrefixAndBasicTriple(t *testing.T) {
	input := `
@prefix foaf: <http://xmlns.com/foaf/0.1/> .
@prefix ex:   <http://example.org/> .

ex:alice foaf:name "Alice" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	tr := triples[0]
	if tr.Subject != "http://example.org/alice" {
		t.Errorf("subject = %q, want http://example.org/alice", tr.Subject)
	}
	if tr.Predicate != "http://xmlns.com/foaf/0.1/name" {
		t.Errorf("predicate = %q, want http://xmlns.com/foaf/0.1/name", tr.Predicate)
	}
	if tr.Object != "Alice" {
		t.Errorf("object = %q, want Alice", tr.Object)
	}
}

func TestTurtleParser_FullIRI(t *testing.T) {
	input := `<http://example.org/alice> <http://xmlns.com/foaf/0.1/name> "Alice" .`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "http://example.org/alice" {
		t.Errorf("subject = %q", triples[0].Subject)
	}
}

func TestTurtleParser_AShorthand(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

ex:Person a owl:Class .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Predicate != "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" {
		t.Errorf("predicate = %q, want rdf:type IRI", triples[0].Predicate)
	}
	if triples[0].Object != "http://www.w3.org/2002/07/owl#Class" {
		t.Errorf("object = %q", triples[0].Object)
	}
}

func TestTurtleParser_PredicateList(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .

ex:Person a owl:Class ;
    rdfs:label "Person" ;
    rdfs:comment "A person" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 3 {
		t.Fatalf("expected 3 triples, got %d", len(triples))
	}
}

func TestTurtleParser_ObjectList(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:bob ex:teaches ex:go101, ex:python101, ex:rust201 .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 3 {
		t.Fatalf("expected 3 triples, got %d", len(triples))
	}
	objects := map[string]bool{}
	for _, tr := range triples {
		objects[tr.Object] = true
	}
	for _, expected := range []string{
		"http://example.org/go101",
		"http://example.org/python101",
		"http://example.org/rust201",
	} {
		if !objects[expected] {
			t.Errorf("missing object %q", expected)
		}
	}
}

func TestTurtleParser_BlankNodeLabel(t *testing.T) {
	input := `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/> .

_:union1 rdf:first ex:Student ;
    rdf:rest _:union2 .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}
	if triples[0].Subject != "_:union1" {
		t.Errorf("subject = %q, want _:union1", triples[0].Subject)
	}
	if triples[1].Object != "_:union2" {
		t.Errorf("object = %q, want _:union2", triples[1].Object)
	}
}

func TestTurtleParser_BlankNodeBrackets(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:alice ex:knows [ ex:name "Bob" ; ex:age "30" ] .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce: alice knows _:tb1, _:tb1 name "Bob", _:tb1 age "30"
	if len(triples) != 3 {
		t.Fatalf("expected 3 triples, got %d", len(triples))
	}
	// The first triple should link alice to the blank node
	found := false
	for _, tr := range triples {
		if tr.Subject == "http://example.org/alice" && tr.Predicate == "http://example.org/knows" {
			found = true
			if !strings.HasPrefix(tr.Object, "_:tb") {
				t.Errorf("object should be blank node, got %q", tr.Object)
			}
		}
	}
	if !found {
		t.Error("missing alice-knows triple")
	}
}

func TestTurtleParser_EmptyBlankNode(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:alice ex:knows [] .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if !strings.HasPrefix(triples[0].Object, "_:tb") {
		t.Errorf("object should be blank node, got %q", triples[0].Object)
	}
}

func TestTurtleParser_TypedLiteral(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:alice ex:score "95"^^xsd:integer .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "95" {
		t.Errorf("object = %q, want 95", triples[0].Object)
	}
}

func TestTurtleParser_LanguageTaggedLiteral(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Person rdfs:label "Person"@en .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "Person" {
		t.Errorf("object = %q, want Person", triples[0].Object)
	}
}

func TestTurtleParser_TripleQuotedString(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Course rdfs:comment """A structured learning experience
with lessons and assessments.""" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	expected := "A structured learning experience\nwith lessons and assessments."
	if triples[0].Object != expected {
		t.Errorf("object = %q, want %q", triples[0].Object, expected)
	}
}

func TestTurtleParser_Comments(t *testing.T) {
	input := `
# This is a comment
@prefix ex: <http://example.org/> .

# Another comment
ex:alice ex:name "Alice" . # inline comment
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
}

func TestTurtleParser_SPARQLStylePrefix(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
PREFIX foaf: <http://xmlns.com/foaf/0.1/>

ex:alice foaf:name "Alice" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "http://example.org/alice" {
		t.Errorf("subject = %q", triples[0].Subject)
	}
}

func TestTurtleParser_BaseDeclaration(t *testing.T) {
	input := `
@base <http://example.org/> .

<alice> <name> "Alice" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "http://example.org/alice" {
		t.Errorf("subject = %q, want http://example.org/alice", triples[0].Subject)
	}
}

func TestTurtleParser_SPARQLStyleBase(t *testing.T) {
	input := `
BASE <http://example.org/>

<alice> <name> "Alice" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "http://example.org/alice" {
		t.Errorf("subject = %q", triples[0].Subject)
	}
}

func TestTurtleParser_EscapeSequences(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:test ex:value "line1\nline2\ttab" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "line1\nline2\ttab" {
		t.Errorf("object = %q", triples[0].Object)
	}
}

func TestTurtleParser_NumericLiteral(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:test ex:value 42 .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "42" {
		t.Errorf("object = %q, want 42", triples[0].Object)
	}
}

func TestTurtleParser_BooleanLiteral(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:test ex:active true .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "true" {
		t.Errorf("object = %q, want true", triples[0].Object)
	}
}

func TestTurtleParser_Graph(t *testing.T) {
	p := &TurtleParser{Graph: "my-graph"}
	input := `
@prefix ex: <http://example.org/> .
ex:alice ex:name "Alice" .
`
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if triples[0].Graph != "my-graph" {
		t.Errorf("graph = %q, want my-graph", triples[0].Graph)
	}
}

func TestTurtleParser_ParseFile(t *testing.T) {
	p := NewTurtleParser()
	triples, err := p.ParseFile("../testdata/edu.ttl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count expected triples from edu.ttl
	if len(triples) < 30 {
		t.Errorf("expected at least 30 triples from edu.ttl, got %d", len(triples))
	}

	// Check specific triples
	found := map[string]bool{
		"Person-class":     false,
		"Student-subclass": false,
		"alice-name":       false,
		"bob-teaches":      false,
		"multiline":        false,
	}

	for _, tr := range triples {
		switch {
		case tr.Subject == "https://ontology.data.schoolai.dev/edu#Person" &&
			tr.Predicate == "http://www.w3.org/1999/02/22-rdf-syntax-ns#type" &&
			tr.Object == "http://www.w3.org/2002/07/owl#Class":
			found["Person-class"] = true
		case tr.Subject == "https://ontology.data.schoolai.dev/edu#Student" &&
			tr.Predicate == "http://www.w3.org/2000/01/rdf-schema#subClassOf" &&
			tr.Object == "https://ontology.data.schoolai.dev/edu#Person":
			found["Student-subclass"] = true
		case tr.Subject == "https://ontology.data.schoolai.dev/edu#alice" &&
			tr.Predicate == "http://xmlns.com/foaf/0.1/name" &&
			tr.Object == "Alice Smith":
			found["alice-name"] = true
		case tr.Subject == "https://ontology.data.schoolai.dev/edu#bob" &&
			tr.Predicate == "https://ontology.data.schoolai.dev/edu#teaches" &&
			tr.Object == "https://ontology.data.schoolai.dev/edu#python101":
			found["bob-teaches"] = true
		case tr.Subject == "https://ontology.data.schoolai.dev/edu#Course" &&
			tr.Predicate == "http://www.w3.org/2000/01/rdf-schema#comment" &&
			strings.Contains(tr.Object, "structured learning"):
			found["multiline"] = true
		}
	}

	for name, ok := range found {
		if !ok {
			t.Errorf("missing expected triple: %s", name)
		}
	}
}

func TestTurtleParser_ImplementsParserInterface(t *testing.T) {
	var _ rdf.Parser = NewTurtleParser()
}

func TestTurtleParser_TrailingSemicolon(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:alice ex:name "Alice" ;
    ex:age "30" ;
.
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}
}

func TestTurtleParser_EmptyInput(t *testing.T) {
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 0 {
		t.Fatalf("expected 0 triples, got %d", len(triples))
	}
}

func TestTurtleParser_OnlyPrefixes(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix foaf: <http://xmlns.com/foaf/0.1/> .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 0 {
		t.Fatalf("expected 0 triples, got %d", len(triples))
	}
}

func TestTurtleParser_DefaultPrefix(t *testing.T) {
	input := `
@prefix : <http://example.org/> .

:alice :name "Alice" .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "http://example.org/alice" {
		t.Errorf("subject = %q", triples[0].Subject)
	}
}

func TestTurtleParser_TypedLiteralWithFullIRI(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .

ex:test ex:value "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "42" {
		t.Errorf("object = %q, want 42", triples[0].Object)
	}
}

func TestTurtleParser_EmptyCollection(t *testing.T) {
	input := `@prefix ex: <http://example.org/> . ex:empty a ex:EmptyCollection . ex:empty ex:items () .`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}
	if triples[1].Predicate != "http://example.org/items" {
		t.Errorf("predicate = %q", triples[1].Predicate)
	}
	if triples[1].Object != "http://www.w3.org/1999/02/22-rdf-syntax-ns#nil" {
		t.Errorf("object = %q, want rdf:nil", triples[1].Object)
	}
}

func TestTurtleParser_Collection(t *testing.T) {
	input := `@prefix ex: <http://example.org/> . ex:foo ex:items (ex:a ex:b ex:c) .`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 7 {
		t.Fatalf("expected 7 triples (1 predicate + 3 rdf:first + 3 rdf:rest), got %d", len(triples))
	}
	var firstCount, restCount int
	for _, tr := range triples {
		if tr.Predicate == "http://www.w3.org/1999/02/22-rdf-syntax-ns#first" {
			firstCount++
		}
		if tr.Predicate == "http://www.w3.org/1999/02/22-rdf-syntax-ns#rest" {
			restCount++
		}
	}
	if firstCount != 3 {
		t.Errorf("expected 3 rdf:first triples, got %d", firstCount)
	}
	if restCount != 3 {
		t.Errorf("expected 3 rdf:rest triples, got %d", restCount)
	}
}

func TestTurtleParser_CollectionInBlankNodeClassExpression(t *testing.T) {
	input := `@prefix sai: <http://example.org/sai/> .
@prefix owl: <http://www.w3.org/2002/07/owl#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

sai:hasSpaceVersion
    a owl:ObjectProperty ;
    rdfs:domain [
        a owl:Class ;
        owl:unionOf (sai:Space sai:SpaceSession)
    ] ;
    rdfs:range sai:SpaceVersion .`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var domainTriple, unionOfTriple *types.Triple
	for i := range triples {
		if triples[i].Predicate == "http://www.w3.org/2000/01/rdf-schema#domain" {
			domainTriple = &triples[i]
		}
		if triples[i].Predicate == "http://www.w3.org/2002/07/owl#unionOf" {
			unionOfTriple = &triples[i]
		}
	}
	if domainTriple == nil {
		t.Fatal("missing domain triple")
	}
	if unionOfTriple == nil {
		t.Fatal("missing owl:unionOf triple")
	}
	if !strings.HasPrefix(unionOfTriple.Object, "_:") {
		t.Errorf("owl:unionOf object should be blank node, got %q", unionOfTriple.Object)
	}
}

func TestTurtleParser_DirectionalLanguageTag(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
ex:doc ex:title "שלום עולם"@he--rtl .
ex:doc ex:desc "Hello world"@en--ltr .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(triples))
	}

	heTriple := triples[0]
	if heTriple.Object != "שלום עולם" {
		t.Errorf("he object = %q, want שלום עולם", heTriple.Object)
	}
	if heTriple.Language != "he" {
		t.Errorf("he language = %q, want he", heTriple.Language)
	}
	if heTriple.Direction != "rtl" {
		t.Errorf("he direction = %q, want rtl", heTriple.Direction)
	}
	if heTriple.Datatype != types.RDFDirLangString {
		t.Errorf("he datatype = %q, want %s", heTriple.Datatype, types.RDFDirLangString)
	}

	enTriple := triples[1]
	if enTriple.Object != "Hello world" {
		t.Errorf("en object = %q, want Hello world", enTriple.Object)
	}
	if enTriple.Language != "en" {
		t.Errorf("en language = %q, want en", enTriple.Language)
	}
	if enTriple.Direction != "ltr" {
		t.Errorf("en direction = %q, want ltr", enTriple.Direction)
	}
}

func TestTurtleParser_LanguageTag(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
ex:doc ex:title "Hello"@en .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Language != "en" {
		t.Errorf("language = %q, want en", triples[0].Language)
	}
	if triples[0].Datatype != types.RDFLangString {
		t.Errorf("datatype = %q, want %s", triples[0].Datatype, types.RDFLangString)
	}
}

func TestTurtleParser_DatatypeLiteral(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:doc ex:count "42"^^xsd:integer .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Object != "42" {
		t.Errorf("object = %q, want 42", triples[0].Object)
	}
	if triples[0].Datatype != types.XSDInteger {
		t.Errorf("datatype = %q, want %s", triples[0].Datatype, types.XSDInteger)
	}
}

func TestTurtleParser_ReifiedTriple(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

ex:s1 ex:p1 ex:o1 .
<<ex:s1 ex:p1 ex:o1>> a rdf:Statement .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundReified := false
	for _, tr := range triples {
		if tr.Predicate == types.RDFSubject || tr.Predicate == types.RDFPredicate || tr.Predicate == types.RDFObject {
			foundReified = true
			if tr.Subject == "" || !strings.HasPrefix(tr.Subject, "_:") {
				t.Errorf("reified triple subject should be blank node, got %q", tr.Subject)
			}
		}
	}
	if !foundReified {
		t.Error("expected reified triple statements, found none")
	}
}

func TestTurtleParser_TripleTerm(t *testing.T) {
	input := `
@prefix ex: <http://example.org/> .
ex:subject ex:hasTriple <<(ex:s ex:p ex:o)>> .
`
	p := NewTurtleParser()
	triples, err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundTripleTerm := false
	for _, tr := range triples {
		if tr.IsTripleTerm {
			foundTripleTerm = true
			break
		}
	}
	if !foundTripleTerm {
		t.Error("expected triple term, found none")
	}
}
