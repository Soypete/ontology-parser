package reasoner

import (
	"context"
	"testing"

	"github.com/soypete/ontology-go/skosast"
	"github.com/soypete/ontology-go/ttlast"
)

func TestReasoner_TransitiveBroader(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/transitive_broader.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := skosast.BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	r := New(doc, hierarchy, "../testdata/reasoner/transitive_broader.ttl")
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	broaderFacts := r.FactSet().FactsByPredicate(skosast.SKOSBroader)
	if len(broaderFacts) == 0 {
		t.Error("Expected broader facts")
	}

	transitiveFacts := r.FactSet().FactsByPredicate(skosast.SKOSBroaderTransitive)
	if len(transitiveFacts) == 0 {
		t.Error("Expected transitive broader facts")
	}
}

func TestReasoner_SymmetricRelated(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/symmetric_related.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := skosast.BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	r := New(doc, hierarchy, "../testdata/reasoner/symmetric_related.ttl")
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	relatedFacts := r.FactSet().FactsByPredicate(skosast.SKOSRelated)
	if len(relatedFacts) < 2 {
		t.Errorf("Expected at least 2 related facts (original + inferred), got %d", len(relatedFacts))
	}
}

func TestReasoner_InconsistencyDetection(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/inconsistency_broader_narrower.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := skosast.BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	r := New(doc, hierarchy, "../testdata/reasoner/inconsistency_broader_narrower.ttl")
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	inconsistencies := r.Inconsistencies()
	if len(inconsistencies) == 0 {
		t.Error("Expected to detect inconsistency")
	}
}

func TestReasoner_CycleDetection(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/cycle_detection.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := skosast.BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	r := New(doc, hierarchy, "../testdata/reasoner/cycle_detection.ttl")
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	inconsistencies := r.Inconsistencies()
	if len(inconsistencies) == 0 {
		t.Error("Expected to detect cycle")
	}

	foundCycle := false
	for _, inc := range inconsistencies {
		if inc.Type == "cycle" {
			foundCycle = true
			break
		}
	}

	if !foundCycle {
		t.Error("Expected cycle type inconsistency")
	}
}

func TestReasoner_FullSKOS(t *testing.T) {
	parser := ttlast.NewParser()
	doc, err := parser.ParseFile("../testdata/reasoner/full_skos.ttl")
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}

	hierarchy, err := skosast.BuildFromDocument(doc)
	if err != nil {
		t.Fatalf("BuildFromDocument error: %v", err)
	}

	r := New(doc, hierarchy, "../testdata/reasoner/full_skos.ttl")
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(r.Facts()) == 0 {
		t.Error("Expected facts from full SKOS")
	}
}

func TestFactSet_FactsByPredicate(t *testing.T) {
	fs := &FactSet{}
	fs.AddFact(Fact{Subject: "s1", Predicate: "broader", Object: "o1"})
	fs.AddFact(Fact{Subject: "s2", Predicate: "related", Object: "o2"})
	fs.AddFact(Fact{Subject: "s3", Predicate: "broader", Object: "o3"})

	broaderFacts := fs.FactsByPredicate("broader")
	if len(broaderFacts) != 2 {
		t.Errorf("Expected 2 broader facts, got %d", len(broaderFacts))
	}
}

func TestFactSet_FactsForSubject(t *testing.T) {
	fs := &FactSet{}
	fs.AddFact(Fact{Subject: "s1", Predicate: "broader", Object: "o1"})
	fs.AddFact(Fact{Subject: "s1", Predicate: "related", Object: "o2"})
	fs.AddFact(Fact{Subject: "s2", Predicate: "broader", Object: "o3"})

	subjectFacts := fs.FactsForSubject("s1")
	if len(subjectFacts) != 2 {
		t.Errorf("Expected 2 facts for s1, got %d", len(subjectFacts))
	}
}
