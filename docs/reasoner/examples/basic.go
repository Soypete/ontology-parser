package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/soypete/ontology-go/reasoning"
	"github.com/soypete/ontology-go/store"
	"github.com/soypete/ontology-go/types"
)

func main() {
	ctx := context.Background()

	// Create a memory triple store
	s := store.NewMemoryStore()

	// Create reasoner with custom logger and max hops
	r := reasoning.New(s,
		reasoning.WithLogger(slog.Default()),
		reasoning.WithMaxHops(2),
	)

	// Load TBox (schema) with class hierarchy
	tboxTriples := []types.Triple{
		// Classes
		{Subject: "ex:Thing", Predicate: types.RDFType, Object: types.RDFSClass},
		{Subject: "ex:Animal", Predicate: types.RDFType, Object: types.RDFSClass},
		{Subject: "ex:Vehicle", Predicate: types.RDFType, Object: types.RDFSClass},
		// Class hierarchy: Dog subclass of Animal
		{Subject: "ex:Dog", Predicate: types.RDFSSubClassOf, Object: "ex:Animal"},
		// Class hierarchy: Car subclass of Vehicle
		{Subject: "ex:Car", Predicate: types.RDFSSubClassOf, Object: "ex:Vehicle"},
		// Property hierarchy
		{Subject: "ex:hasOwner", Predicate: types.RDFSSubPropertyOf, Object: "ex:hasRelation"},
	}

	if err := r.LoadTBoxData(ctx, tboxTriples); err != nil {
		slog.Error("failed to load TBox", "error", err)
		return
	}

	// Load ABox (instances)
	aboxTriples := []types.Triple{
		{Subject: "ex:fido", Predicate: types.RDFType, Object: "ex:Dog"},
		{Subject: "ex:fido", Predicate: "ex:hasName", Object: "\"Fido\"", IsLiteral: true},
		{Subject: "ex:alice", Predicate: "ex:owns", Object: "ex:fido"},
	}

	if err := r.LoadABoxData(ctx, aboxTriples); err != nil {
		slog.Error("failed to load ABox", "error", err)
		return
	}

	// Perform reasoning to infer relationships
	if err := r.Reason(ctx); err != nil {
		slog.Error("reasoning failed", "error", err)
		return
	}

	// Get all inferred triples
	inferred := r.Inferred()
	fmt.Printf("Inferred %d triples:\n", len(inferred))
	for _, t := range inferred {
		fmt.Printf("  %s -> %s -> %s (graph: %s)\n",
			t.Subject, t.Predicate, t.Object, t.Graph)
	}

	// Query inferred triples by subject
	dogInferred := r.GetInferredBySubject("ex:Dog")
	fmt.Printf("\nInferred triples about ex:Dog: %d\n", len(dogInferred))

	// Find related entities (2-hop traversal)
	related := r.FindRelated(ctx, "ex:alice", 2)
	fmt.Printf("\nEntities related to ex:alice within 2 hops: %d\n", len(related))
	for _, t := range related {
		fmt.Printf("  %s -> %s -> %s\n", t.Subject, t.Predicate, t.Object)
	}
}
