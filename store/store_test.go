package store

import (
	"os"
	"sort"
	"testing"

	"github.com/soypete/ontology-go/rdf"
	"github.com/soypete/ontology-go/types"
)

func TestMemoryStore_Register(t *testing.T) {
	s := NewMemoryStore()

	triples := []types.Triple{
		{Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/name", Object: "Alice"},
		{Subject: "http://example.org/alice", Predicate: "http://xmlns.com/foaf/0.1/age", Object: "30"},
	}

	err := s.Register("people", triples)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 triples, got %d", len(all))
	}

	// Graph should be set
	for _, tr := range all {
		if tr.Graph != "people" {
			t.Errorf("expected graph 'people', got %q", tr.Graph)
		}
	}
}

func TestMemoryStore_RegisterReplace(t *testing.T) {
	s := NewMemoryStore()

	triples1 := []types.Triple{
		{Subject: "s1", Predicate: "p1", Object: "o1"},
	}
	triples2 := []types.Triple{
		{Subject: "s2", Predicate: "p2", Object: "o2"},
		{Subject: "s3", Predicate: "p3", Object: "o3"},
	}

	_ = s.Register("graph1", triples1)
	_ = s.Register("graph1", triples2) // replace

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 triples after replace, got %d", len(all))
	}

	if all[0].Subject != "s2" {
		t.Errorf("expected s2, got %q", all[0].Subject)
	}
}

func TestMemoryStore_RegisterEmptyName(t *testing.T) {
	s := NewMemoryStore()
	err := s.Register("", nil)
	if err == nil {
		t.Error("expected error for empty graph name")
	}
}

func TestMemoryStore_Remove(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g1", []types.Triple{{Subject: "s1", Predicate: "p1", Object: "o1"}})
	_ = s.Register("g2", []types.Triple{{Subject: "s2", Predicate: "p2", Object: "o2"}})

	err := s.Remove("g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := s.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(all))
	}

	if all[0].Subject != "s2" {
		t.Errorf("expected s2 remaining, got %q", all[0].Subject)
	}
}

func TestMemoryStore_RemoveNotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for removing nonexistent graph")
	}
}

func TestMemoryStore_List(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("alpha", []types.Triple{{Subject: "s", Predicate: "p", Object: "o"}})
	_ = s.Register("beta", []types.Triple{{Subject: "s", Predicate: "p", Object: "o"}})

	names := s.List()
	sort.Strings(names)

	if len(names) != 2 {
		t.Fatalf("expected 2 graphs, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("unexpected graph names: %v", names)
	}
}

func TestMemoryStore_MatchSubject(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g", []types.Triple{
		{Subject: "http://example.org/alice", Predicate: "name", Object: "Alice"},
		{Subject: "http://example.org/alice", Predicate: "age", Object: "30"},
		{Subject: "http://example.org/bob", Predicate: "name", Object: "Bob"},
	})

	matches := s.Match("http://example.org/alice", "", "")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestMemoryStore_MatchPredicate(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g", []types.Triple{
		{Subject: "s1", Predicate: "name", Object: "Alice"},
		{Subject: "s2", Predicate: "name", Object: "Bob"},
		{Subject: "s1", Predicate: "age", Object: "30"},
	})

	matches := s.Match("", "name", "")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestMemoryStore_MatchObject(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g", []types.Triple{
		{Subject: "s1", Predicate: "p1", Object: "Alice"},
		{Subject: "s2", Predicate: "p2", Object: "Alice"},
		{Subject: "s3", Predicate: "p3", Object: "Bob"},
	})

	matches := s.Match("", "", "Alice")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestMemoryStore_MatchWildcard(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g", []types.Triple{
		{Subject: "s1", Predicate: "p1", Object: "o1"},
		{Subject: "s2", Predicate: "p2", Object: "o2"},
	})

	matches := s.Match("", "", "")
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches for wildcard, got %d", len(matches))
	}
}

func TestMemoryStore_MatchCombined(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g", []types.Triple{
		{Subject: "s1", Predicate: "name", Object: "Alice"},
		{Subject: "s1", Predicate: "age", Object: "30"},
		{Subject: "s2", Predicate: "name", Object: "Bob"},
	})

	matches := s.Match("s1", "name", "")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	if matches[0].Object != "Alice" {
		t.Errorf("expected Alice, got %q", matches[0].Object)
	}
}

func TestMemoryStore_MultipleGraphs(t *testing.T) {
	s := NewMemoryStore()

	_ = s.Register("g1", []types.Triple{
		{Subject: "s1", Predicate: "p1", Object: "o1"},
	})
	_ = s.Register("g2", []types.Triple{
		{Subject: "s2", Predicate: "p2", Object: "o2"},
	})

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 triples from 2 graphs, got %d", len(all))
	}

	// Verify graph names
	graphs := make(map[string]bool)
	for _, tr := range all {
		graphs[tr.Graph] = true
	}
	if !graphs["g1"] || !graphs["g2"] {
		t.Errorf("expected both graphs, got %v", graphs)
	}
}

func TestMemoryStore_WithParsedFixture(t *testing.T) {
	f, err := os.Open("../testdata/foaf.rdf")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	parser := rdf.NewXMLParser("foaf")
	triples, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	s := NewMemoryStore()
	err = s.Register("foaf", triples)
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Should be able to find Alice by name
	matches := s.Match("http://example.org/people/alice", "http://xmlns.com/foaf/0.1/name", "")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for Alice name, got %d", len(matches))
	}
	if matches[0].Object != "Alice Smith" {
		t.Errorf("expected Alice Smith, got %q", matches[0].Object)
	}

	// Should find all foaf:knows relationships
	matches = s.Match("http://example.org/people/alice", "http://xmlns.com/foaf/0.1/knows", "")
	if len(matches) != 2 {
		t.Fatalf("expected 2 knows matches, got %d", len(matches))
	}
}
