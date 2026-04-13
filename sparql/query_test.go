package query

import (
	"os"
	"testing"

	"github.com/soypete/ontology-go/rdf"
	"github.com/soypete/ontology-go/store"
	"github.com/soypete/ontology-go/types"
)

func setupFOAFStore(t *testing.T) store.Store {
	t.Helper()

	f, err := os.Open("../testdata/foaf.rdf")
	if err != nil {
		t.Fatalf("failed to open foaf fixture: %v", err)
	}
	defer f.Close()

	parser := rdf.NewXMLParser("foaf")
	triples, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse foaf: %v", err)
	}

	s := store.NewMemoryStore()
	if err := s.Register("foaf", triples); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	return s
}

func setupWikidataStore(t *testing.T) store.Store {
	t.Helper()

	f, err := os.Open("../testdata/wikidata_sample.rdf")
	if err != nil {
		t.Fatalf("failed to open wikidata fixture: %v", err)
	}
	defer f.Close()

	parser := rdf.NewXMLParser("wikidata")
	triples, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse wikidata: %v", err)
	}

	s := store.NewMemoryStore()
	if err := s.Register("wikidata", triples); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	return s
}

func TestEngine_SimpleSelect(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Alice"},
		{Subject: "http://example.org/bob", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?person ?name WHERE {
			?person foaf:name ?name .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}

	// Verify bindings contain expected values
	names := make(map[string]string)
	for _, b := range result.Bindings {
		names[b["person"]] = b["name"]
	}

	if names["http://example.org/alice"] != "Alice" {
		t.Errorf("expected Alice, got %q", names["http://example.org/alice"])
	}
	if names["http://example.org/bob"] != "Bob" {
		t.Errorf("expected Bob, got %q", names["http://example.org/bob"])
	}

	// Check that triples were recorded
	if len(result.Triples) == 0 {
		t.Error("expected matched triples, got none")
	}

	// Check path
	if len(result.Path) == 0 {
		t.Error("expected traversal path, got none")
	}
}

func TestEngine_SelectStar(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "s1", Predicate: "p1", Object: "o1"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`SELECT * WHERE { ?s ?p ?o }`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	b := result.Bindings[0]
	if b["s"] != "s1" || b["p"] != "p1" || b["o"] != "o1" {
		t.Errorf("unexpected binding: %v", b)
	}
}

func TestEngine_JoinPatterns(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice"},
		{Subject: "alice", Predicate: "knows", Object: "bob"},
		{Subject: "bob", Predicate: "name", Object: "Bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?name WHERE {
			?person knows ?friend .
			?friend name ?name .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	if result.Bindings[0]["name"] != "Bob" {
		t.Errorf("expected Bob, got %q", result.Bindings[0]["name"])
	}
}

func TestEngine_FilterEquals(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice"},
		{Subject: "bob", Predicate: "name", Object: "Bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person WHERE {
			?person name ?name .
			FILTER (?name = "Alice")
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	if result.Bindings[0]["person"] != "alice" {
		t.Errorf("expected alice, got %q", result.Bindings[0]["person"])
	}
}

func TestEngine_FilterNotEquals(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice"},
		{Subject: "bob", Predicate: "name", Object: "Bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person WHERE {
			?person name ?name .
			FILTER (?name != "Alice")
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	if result.Bindings[0]["person"] != "bob" {
		t.Errorf("expected bob, got %q", result.Bindings[0]["person"])
	}
}

func TestEngine_FilterRegex(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice Smith"},
		{Subject: "bob", Predicate: "name", Object: "Bob Jones"},
		{Subject: "charlie", Predicate: "name", Object: "Alice Johnson"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person ?name WHERE {
			?person name ?name .
			FILTER (regex(?name, "^Alice"))
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}
}

func TestEngine_FilterContains(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice Smith"},
		{Subject: "bob", Predicate: "name", Object: "Robert Jones"},
		{Subject: "charlie", Predicate: "name", Object: "Alice Johnson"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person ?name WHERE {
			?person name ?name .
			FILTER (contains(?name, "Alice"))
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}
}

func TestEngine_FilterStartsWith(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice Smith"},
		{Subject: "bob", Predicate: "name", Object: "Robert Jones"},
		{Subject: "charlie", Predicate: "name", Object: "Alice Johnson"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person ?name WHERE {
			?person name ?name .
			FILTER (startsWith(?name, "Alice"))
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}
}

func TestEngine_FilterEndsWith(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice Smith"},
		{Subject: "bob", Predicate: "name", Object: "Bob Jones"},
		{Subject: "charlie", Predicate: "name", Object: "Charlie Brown"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person ?name WHERE {
			?person name ?name .
			FILTER (endsWith(?name, "Jones"))
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestEngine_FilterIsURI(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Alice"},
		{Subject: "bob", Predicate: "name", Object: "Bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?s WHERE {
			?s ?p ?o .
			FILTER (isURI(?s))
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	if result.Bindings[0]["s"] != "http://example.org/alice" {
		t.Errorf("expected http://example.org/alice, got %s", result.Bindings[0]["s"])
	}
}

func TestEngine_FilterIsLiteral(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Alice"},
		{Subject: "bob", Predicate: "name", Object: "Bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?o WHERE {
			?s ?p ?o .
			FILTER (isLiteral(?o))
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}
}

func TestEngine_Limit(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "s1", Predicate: "p", Object: "o1"},
		{Subject: "s2", Predicate: "p", Object: "o2"},
		{Subject: "s3", Predicate: "p", Object: "o3"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`SELECT ?s WHERE { ?s p ?o } LIMIT 2`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}
}

func TestEngine_Offset(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "s1", Predicate: "p", Object: "o1"},
		{Subject: "s2", Predicate: "p", Object: "o2"},
		{Subject: "s3", Predicate: "p", Object: "o3"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`SELECT ?s WHERE { ?s p ?o } LIMIT 1 OFFSET 1`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestEngine_Distinct(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "knows", Object: "bob"},
		{Subject: "alice", Predicate: "likes", Object: "bob"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`SELECT DISTINCT ?person WHERE { ?person ?p ?o }`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 distinct binding, got %d", len(result.Bindings))
	}

	if result.Bindings[0]["person"] != "alice" {
		t.Errorf("expected alice, got %q", result.Bindings[0]["person"])
	}
}

func TestEngine_Optional(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice"},
		{Subject: "alice", Predicate: "email", Object: "alice@example.org"},
		{Subject: "bob", Predicate: "name", Object: "Bob"},
		// Bob has no email
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		SELECT ?person ?name ?email WHERE {
			?person name ?name .
			OPTIONAL { ?person email ?email }
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(result.Bindings))
	}

	// Find Alice and Bob
	for _, b := range result.Bindings {
		switch b["name"] {
		case "Alice":
			if b["email"] != "alice@example.org" {
				t.Errorf("Alice should have email, got %q", b["email"])
			}
		case "Bob":
			if b["email"] != "" {
				t.Errorf("Bob should have no email, got %q", b["email"])
			}
		default:
			t.Errorf("unexpected name: %q", b["name"])
		}
	}
}

func TestEngine_Prefix(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Alice"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			ex:alice foaf:name ?name .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	if result.Bindings[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %q", result.Bindings[0]["name"])
	}
}

func TestEngine_RDFTypeShorthand(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", Object: "http://xmlns.com/foaf/0.1/Person"},
		{Subject: "acme", Predicate: "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", Object: "http://xmlns.com/foaf/0.1/Organization"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?s WHERE { ?s a foaf:Person }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
	if result.Bindings[0]["s"] != "alice" {
		t.Errorf("expected alice, got %q", result.Bindings[0]["s"])
	}
}

func TestEngine_FOAFFixtureQuery(t *testing.T) {
	s := setupFOAFStore(t)
	engine := NewEngine(s)

	// Find all people Alice knows
	result, err := engine.Execute(`
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name WHERE {
			<http://example.org/people/alice> foaf:knows ?person .
			?person foaf:name ?name .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 2 {
		t.Fatalf("expected 2 people Alice knows, got %d", len(result.Bindings))
	}

	names := make(map[string]bool)
	for _, b := range result.Bindings {
		names[b["name"]] = true
	}
	if !names["Bob Jones"] || !names["Charlie Brown"] {
		t.Errorf("expected Bob Jones and Charlie Brown, got %v", names)
	}
}

func TestEngine_WikidataFixtureQuery(t *testing.T) {
	s := setupWikidataStore(t)
	engine := NewEngine(s)

	// Find Douglas Adams's occupations
	result, err := engine.Execute(`
		PREFIX wdt: <http://www.wikidata.org/prop/direct/>
		PREFIX wd: <http://www.wikidata.org/entity/>
		PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
		SELECT ?occupation ?label WHERE {
			wd:Q42 wdt:P106 ?occupation .
			?occupation rdfs:label ?label .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) < 2 {
		t.Fatalf("expected at least 2 occupation bindings, got %d", len(result.Bindings))
	}

	labels := make(map[string]bool)
	for _, b := range result.Bindings {
		labels[b["label"]] = true
	}
	if !labels["writer"] {
		t.Errorf("expected 'writer' among labels, got %v", labels)
	}
}

func TestEngine_InvalidQuery(t *testing.T) {
	s := store.NewMemoryStore()
	engine := NewEngine(s)

	_, err := engine.Execute("not a valid query")
	if err == nil {
		t.Error("expected error for invalid query")
	}
}

func TestEngine_EmptyResult(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Register("test", []types.Triple{
		{Subject: "alice", Predicate: "name", Object: "Alice"},
	})

	engine := NewEngine(s)
	result, err := engine.Execute(`SELECT ?s WHERE { ?s nonexistent ?o }`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bindings) != 0 {
		t.Errorf("expected 0 bindings, got %d", len(result.Bindings))
	}
}

func TestParse_Basic(t *testing.T) {
	q, err := Parse(`
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name WHERE {
			?person foaf:name ?name .
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Type != QuerySelect {
		t.Errorf("expected SELECT, got %d", q.Type)
	}
	if len(q.Variables) != 1 || q.Variables[0] != "name" {
		t.Errorf("expected [name], got %v", q.Variables)
	}
	if len(q.Where) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(q.Where))
	}
	if q.Prefixes["foaf"] != "http://xmlns.com/foaf/0.1/" {
		t.Errorf("unexpected prefix: %v", q.Prefixes)
	}
}

func TestParse_LimitOffset(t *testing.T) {
	q, err := Parse(`SELECT ?s WHERE { ?s ?p ?o } LIMIT 10 OFFSET 5`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Limit != 10 {
		t.Errorf("expected LIMIT 10, got %d", q.Limit)
	}
	if q.Offset != 5 {
		t.Errorf("expected OFFSET 5, got %d", q.Offset)
	}
}

func TestParse_NonSelectError(t *testing.T) {
	_, err := Parse(`CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`)
	if err == nil {
		t.Error("expected error for CONSTRUCT query")
	}
}
