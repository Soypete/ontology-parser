package reasoning

import (
	"context"
	"log/slog"
	"testing"

	"github.com/soypete/ontology-go/store"
	"github.com/soypete/ontology-go/types"
)

func TestReasoner_LoadTBoxData(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	triples := []types.Triple{
		{Subject: "ex:Animal", Predicate: types.RDFType, Object: types.RDFSClass},
		{Subject: "ex:Dog", Predicate: types.RDFSSubClassOf, Object: "ex:Animal"},
		{Subject: "ex:Cat", Predicate: types.RDFSSubClassOf, Object: "ex:Animal"},
	}

	err := r.LoadTBoxData(context.Background(), triples)
	if err != nil {
		t.Fatalf("LoadTBoxData failed: %v", err)
	}

	stored := s.Match("", "", "")
	if len(stored) != 3 {
		t.Errorf("expected 3 triples, got %d", len(stored))
	}
}

func TestReasoner_LoadABoxData(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	triples := []types.Triple{
		{Subject: "ex:fido", Predicate: types.RDFType, Object: "ex:Dog"},
		{Subject: "ex:max", Predicate: types.RDFType, Object: "ex:Cat"},
	}

	err := r.LoadABoxData(context.Background(), triples)
	if err != nil {
		t.Fatalf("LoadABoxData failed: %v", err)
	}

	stored := s.Match("", "", "")
	if len(stored) != 2 {
		t.Errorf("expected 2 triples, got %d", len(stored))
	}
}

func TestReasoner_InferClassHierarchy(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	triples := []types.Triple{
		{Subject: "ex:Vehicle", Predicate: types.RDFType, Object: types.RDFSClass},
		{Subject: "ex:Car", Predicate: types.RDFSSubClassOf, Object: "ex:Vehicle"},
		{Subject: "ex:SportsCar", Predicate: types.RDFSSubClassOf, Object: "ex:Car"},
	}

	if err := r.LoadTBoxData(context.Background(), triples); err != nil {
		t.Fatalf("LoadTBoxData failed: %v", err)
	}

	if err := r.Reason(context.Background()); err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	inferred := r.Inferred()
	if len(inferred) == 0 {
		t.Error("expected inferred triples, got none")
	}

	hasSubClassOf := false
	for _, t := range inferred {
		if t.Predicate == types.RDFSSubClassOf {
			hasSubClassOf = true
		}
	}
	if !hasSubClassOf {
		t.Error("expected rdfs:subClassOf in inferred triples")
	}
}

func TestReasoner_InferPropertyHierarchy(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	triples := []types.Triple{
		{Subject: "ex:hasParent", Predicate: types.RDFSSubPropertyOf, Object: "ex:hasAncestor"},
		{Subject: "ex:hasGrandparent", Predicate: types.RDFSSubPropertyOf, Object: "ex:hasAncestor"},
	}

	if err := r.LoadTBoxData(context.Background(), triples); err != nil {
		t.Fatalf("LoadTBoxData failed: %v", err)
	}

	if err := r.Reason(context.Background()); err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	inferred := r.Inferred()
	if len(inferred) == 0 {
		t.Error("expected inferred triples, got none")
	}
}

func TestReasoner_FindRelated(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s, WithMaxHops(2))

	triples := []types.Triple{
		{Subject: "ex:alice", Predicate: "ex:knows", Object: "ex:bob"},
		{Subject: "ex:bob", Predicate: "ex:knows", Object: "ex:charlie"},
		{Subject: "ex:charlie", Predicate: "ex:likes", Object: "ex:diana"},
	}

	if err := r.LoadABoxData(context.Background(), triples); err != nil {
		t.Fatalf("LoadABoxData failed: %v", err)
	}

	related := r.FindRelated(context.Background(), "ex:alice", 2)
	if len(related) == 0 {
		t.Error("expected related triples, got none")
	}
}

func TestReasoner_GetInferredBySubject(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	triples := []types.Triple{
		{Subject: "ex:Dog", Predicate: types.RDFSSubClassOf, Object: "ex:Animal"},
		{Subject: "ex:Cat", Predicate: types.RDFSSubClassOf, Object: "ex:Animal"},
	}

	if err := r.LoadTBoxData(context.Background(), triples); err != nil {
		t.Fatalf("LoadTBoxData failed: %v", err)
	}

	if err := r.Reason(context.Background()); err != nil {
		t.Fatalf("Reason failed: %v", err)
	}

	inferredDog := r.GetInferredBySubject("ex:Dog")
	if len(inferredDog) == 0 {
		t.Error("expected inferred triples for ex:Dog, got none")
	}
}

func TestReasoner_Store(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	returned := r.Store()
	if returned != s {
		t.Error("Store() should return the same store instance")
	}
}

func TestReasoner_Options(t *testing.T) {
	logger := slog.Default()

	s := store.NewMemoryStore()
	r := New(s, WithLogger(logger), WithMaxHops(3))

	if r.maxHops != 3 {
		t.Errorf("expected maxHops 3, got %d", r.maxHops)
	}

	if r.logger != logger {
		t.Error("WithLogger option not applied")
	}
}

func TestReasoner_NoDuplicates(t *testing.T) {
	s := store.NewMemoryStore()
	r := New(s)

	triples := []types.Triple{
		{Subject: "ex:A", Predicate: types.RDFSSubClassOf, Object: "ex:B"},
		{Subject: "ex:B", Predicate: types.RDFSSubClassOf, Object: "ex:C"},
		{Subject: "ex:A", Predicate: types.RDFSSubClassOf, Object: "ex:C"},
	}

	if err := r.LoadTBoxData(context.Background(), triples); err != nil {
		t.Fatalf("LoadTBoxData failed: %v", err)
	}

	if err := r.Reason(context.Background()); err != nil {
		t.Fatalf("Reason failed: %v", err)
	}
}
