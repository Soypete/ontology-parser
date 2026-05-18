package viz

import (
	"testing"

	"github.com/soypete/ontology-go/types"
)

func TestGraph_ProcessTriples_Classes(t *testing.T) {
	g := NewGraph()

	triples := []types.Triple{
		{Subject: "http://example.org/Person", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
		{Subject: "http://example.org/Person", Predicate: types.RDFSLabel, Object: "Person", IsLiteral: true},
		{Subject: "http://example.org/Student", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
		{Subject: "http://example.org/Student", Predicate: types.RDFSSubClassOf, Object: "http://example.org/Person", IsLiteral: false},
	}

	g.ProcessTriples(triples)

	// Should have Person, Student, and owl:Class nodes
	if len(g.Nodes) < 2 {
		t.Errorf("expected at least 2 nodes, got %d", len(g.Nodes))
	}

	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}

	// Check that Person has correct label
	foundPerson := false
	for _, n := range g.Nodes {
		if n.ID == "http://example.org/Person" && n.Label == "Person" {
			foundPerson = true
		}
	}
	if !foundPerson {
		t.Error("expected Person node with label 'Person'")
	}
}

func TestGraph_ProcessTriples_Properties(t *testing.T) {
	g := NewGraph()

	triples := []types.Triple{
		{Subject: "http://example.org/hasName", Predicate: types.RDFType, Object: types.OWLObjectProperty, IsLiteral: false},
		{Subject: "http://example.org/hasAge", Predicate: types.RDFType, Object: types.OWLDatatypeProperty, IsLiteral: false},
	}

	g.ProcessTriples(triples)

	if len(g.Nodes) < 2 {
		t.Errorf("expected at least 2 nodes, got %d", len(g.Nodes))
	}

	foundTypes := map[string]bool{}
	for _, n := range g.Nodes {
		foundTypes[n.Type] = true
	}

	if !foundTypes["objectProperty"] {
		t.Error("expected objectProperty type")
	}
	if !foundTypes["datatypeProperty"] {
		t.Error("expected datatypeProperty type")
	}
}

func TestGraph_NamespaceExtraction(t *testing.T) {
	g := NewGraph()

	triples := []types.Triple{
		{Subject: "http://example.org/Person", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
		{Subject: "http://schema.org/Person", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
	}

	g.ProcessTriples(triples)

	// Should have namespaces for example.org and schema.org
	if len(g.Namespaces) < 2 {
		t.Errorf("expected at least 2 namespaces, got %d: %v", len(g.Namespaces), g.Namespaces)
	}
}

func TestGraph_SkosRelationships(t *testing.T) {
	g := NewGraph()

	triples := []types.Triple{
		{Subject: "http://example.org/Course", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
		{Subject: "http://example.org/OnlineCourse", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
		{Subject: "http://example.org/OnlineCourse", Predicate: "http://www.w3.org/2004/02/skos/core#broader", Object: "http://example.org/Course", IsLiteral: false},
	}

	g.ProcessTriples(triples)

	found := false
	for _, e := range g.Edges {
		if e.Label == "skos:broader" {
			found = true
		}
	}
	if !found {
		t.Error("expected skos:broader edge")
	}
}

func TestGraph_ToJSON(t *testing.T) {
	g := NewGraph()

	triples := []types.Triple{
		{Subject: "http://example.org/Person", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
	}

	g.ProcessTriples(triples)

	json := string(g.ToJSON())
	if len(json) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestGraph_DomainRangeEdges(t *testing.T) {
	g := NewGraph()

	triples := []types.Triple{
		{Subject: "http://example.org/hasSyllabus", Predicate: types.RDFType, Object: types.OWLObjectProperty, IsLiteral: false},
		{Subject: "http://example.org/hasSyllabus", Predicate: types.RDFSdomain, Object: "http://example.org/Course", IsLiteral: false},
		{Subject: "http://example.org/hasSyllabus", Predicate: types.RDFSRange, Object: "http://example.org/Syllabus", IsLiteral: false},
		{Subject: "http://example.org/Course", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
		{Subject: "http://example.org/Syllabus", Predicate: types.RDFType, Object: types.OWLClass, IsLiteral: false},
	}

	g.ProcessTriples(triples)

	domainEdges := 0
	rangeEdges := 0
	for _, e := range g.Edges {
		if e.Label == "rdfs:domain" {
			domainEdges++
		}
		if e.Label == "rdfs:range" {
			rangeEdges++
		}
	}

	if domainEdges != 1 {
		t.Errorf("expected 1 rdfs:domain edge, got %d", domainEdges)
	}
	if rangeEdges != 1 {
		t.Errorf("expected 1 rdfs:range edge, got %d", rangeEdges)
	}

	nodeLabels := map[string]string{}
	for _, n := range g.Nodes {
		nodeLabels[n.Label] = n.ID
	}

	if nodeLabels["Course"] == "" {
		t.Error("expected Course node to exist")
	}
	if nodeLabels["Syllabus"] == "" {
		t.Error("expected Syllabus node to exist")
	}
	if nodeLabels["hasSyllabus"] == "" {
		t.Error("expected hasSyllabus node to exist")
	}
}
