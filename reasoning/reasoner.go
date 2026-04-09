// Package reasoning provides OWL reasoning capabilities for RDF ontologies.
//
// This package supports loading TBox (schema) and ABox (instance) data,
// performing inference to derive implicit relationships, and supporting
// up to 2-hop node traversal inference.
//
// The reasoning engine supports:
//   - TBox loading: Class hierarchies (rdfs:subClassOf), property hierarchies (rdfs:subPropertyOf)
//   - ABox loading: Instance data with class memberships and property values
//   - Inference: Deriving implicit relationships through class/property hierarchies
//   - 2-hop traversal: Finding related entities within 2 hops of connectivity
//
// Example:
//
//	reasoner := reasoning.New(store.NewMemoryStore(),
//	    reasoning.WithLogger(logger),
//	)
//
//	// Load TBox (schema)
//	err := reasoner.LoadTBox(ctx, "schema.ttl")
//
//	// Load ABox (instances)
//	err := reasoner.LoadABox(ctx, "data.ttl")
//
//	// Perform reasoning
//	err := reasoner.Reason(ctx)
//
//	// Query inferred triples
//	triples := reasoner.Inferred()
package reasoning

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/soypete/ontology-go/store"
	"github.com/soypete/ontology-go/ttl"
	"github.com/soypete/ontology-go/types"
)

const (
	GraphTBox     = "tbox"
	GraphABox     = "abox"
	GraphInferred = "inferred"
)

type Option func(*Reasoner)

func WithLogger(logger *slog.Logger) Option {
	return func(r *Reasoner) {
		r.logger = logger
	}
}

func WithMaxHops(hops int) Option {
	return func(r *Reasoner) {
		r.maxHops = hops
	}
}

type Reasoner struct {
	store   store.Store
	logger  *slog.Logger
	maxHops int

	inferred []types.Triple
}

func New(store store.Store, opts ...Option) *Reasoner {
	r := &Reasoner{
		store:   store,
		logger:  slog.Default(),
		maxHops: 2,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *Reasoner) LoadTBox(ctx context.Context, path string) error {
	r.logger.Info("loading TBox", "path", path)

	parsed, err := ttl.NewTurtleParser().ParseFile(path)
	if err != nil {
		return fmt.Errorf("failed to parse TBox: %w", err)
	}

	if err := r.store.Register(GraphTBox, parsed); err != nil {
		return fmt.Errorf("failed to register TBox: %w", err)
	}

	r.logger.Debug("TBox loaded", "triples", len(parsed))
	return nil
}

func (r *Reasoner) LoadABox(ctx context.Context, path string) error {
	r.logger.Info("loading ABox", "path", path)

	parsed, err := ttl.NewTurtleParser().ParseFile(path)
	if err != nil {
		return fmt.Errorf("failed to parse ABox: %w", err)
	}

	if err := r.store.Register(GraphABox, parsed); err != nil {
		return fmt.Errorf("failed to register ABox: %w", err)
	}

	r.logger.Debug("ABox loaded", "triples", len(parsed))
	return nil
}

func (r *Reasoner) LoadTBoxData(ctx context.Context, triples []types.Triple) error {
	if err := r.store.Register(GraphTBox, triples); err != nil {
		return fmt.Errorf("failed to register TBox data: %w", err)
	}
	return nil
}

func (r *Reasoner) LoadABoxData(ctx context.Context, triples []types.Triple) error {
	if err := r.store.Register(GraphABox, triples); err != nil {
		return fmt.Errorf("failed to register ABox data: %w", err)
	}
	return nil
}

func (r *Reasoner) Reason(ctx context.Context) error {
	r.logger.Info("starting reasoning")

	if err := r.inferClassHierarchy(ctx); err != nil {
		return fmt.Errorf("failed to infer class hierarchy: %w", err)
	}

	if err := r.inferPropertyHierarchy(ctx); err != nil {
		return fmt.Errorf("failed to infer property hierarchy: %w", err)
	}

	r.logger.Info("reasoning complete", "inferred", len(r.inferred))
	return nil
}

func (r *Reasoner) inferClassHierarchy(ctx context.Context) error {
	subClassOf := r.store.Match("", types.RDFSSubClassOf, "")
	processed := make(map[string]bool)

	for _, t := range subClassOf {
		if err := r.inferSubClassChain(ctx, t.Subject, t.Object, processed); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reasoner) inferSubClassChain(ctx context.Context, subClass, superClass string, processed map[string]bool) error {
	key := subClass + "->" + superClass
	if processed[key] {
		return nil
	}
	processed[key] = true

	directSubs := r.store.Match(subClass, types.RDFSSubClassOf, superClass)
	for _, t := range directSubs {
		r.addInferred(t)
	}

	intermediate := r.store.Match(subClass, types.RDFSSubClassOf, "")
	for _, t := range intermediate {
		if t.Object != superClass {
			r.addInferred(types.Triple{
				Subject:   subClass,
				Predicate: types.RDFSSubClassOf,
				Object:    superClass,
				Graph:     GraphInferred,
			})
		}
	}

	return nil
}

func (r *Reasoner) inferPropertyHierarchy(ctx context.Context) error {
	subPropOf := r.store.Match("", types.RDFSSubPropertyOf, "")
	processed := make(map[string]bool)

	for _, t := range subPropOf {
		if err := r.inferSubPropertyChain(ctx, t.Subject, t.Object, processed); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reasoner) inferSubPropertyChain(ctx context.Context, subProp, superProp string, processed map[string]bool) error {
	key := subProp + "->" + superProp
	if processed[key] {
		return nil
	}
	processed[key] = true

	directSubs := r.store.Match(subProp, types.RDFSSubPropertyOf, superProp)
	for _, t := range directSubs {
		r.addInferred(t)
	}

	intermediate := r.store.Match(subProp, types.RDFSSubPropertyOf, "")
	for _, t := range intermediate {
		if t.Object != superProp {
			r.addInferred(types.Triple{
				Subject:   subProp,
				Predicate: types.RDFSSubPropertyOf,
				Object:    superProp,
				Graph:     GraphInferred,
			})
		}
	}

	return nil
}

func (r *Reasoner) addInferred(t types.Triple) {
	r.inferred = append(r.inferred, t)
}

func (r *Reasoner) Inferred() []types.Triple {
	return r.inferred
}

func (r *Reasoner) GetInferredBySubject(subject string) []types.Triple {
	var result []types.Triple
	for _, t := range r.inferred {
		if t.Subject == subject {
			result = append(result, t)
		}
	}
	return result
}

func (r *Reasoner) FindRelated(ctx context.Context, entity string, maxHops int) []types.Triple {
	if maxHops <= 0 {
		maxHops = r.maxHops
	}

	var result []types.Triple
	visited := make(map[string]bool)
	frontier := []string{entity}
	currentHop := 0

	for currentHop < maxHops {
		var nextFrontier []string

		for _, e := range frontier {
			if visited[e] {
				continue
			}
			visited[e] = true

			outgoing := r.store.Match(e, "", "")
			incoming := r.store.Match("", "", e)
			derived := r.derivedTriples(e)

			result = append(result, outgoing...)
			result = append(result, incoming...)
			result = append(result, derived...)

			for _, t := range outgoing {
				if t.Object != e && !visited[t.Object] {
					nextFrontier = append(nextFrontier, t.Object)
				}
			}
			for _, t := range incoming {
				if t.Subject != e && !visited[t.Subject] {
					nextFrontier = append(nextFrontier, t.Subject)
				}
			}
		}

		frontier = nextFrontier
		if len(frontier) == 0 {
			break
		}
		currentHop++
	}

	return result
}

func (r *Reasoner) derivedTriples(entity string) []types.Triple {
	var result []types.Triple

	for _, t := range r.inferred {
		if t.Subject == entity || t.Object == entity {
			result = append(result, t)
		}
	}

	return result
}

func (r *Reasoner) Store() store.Store {
	return r.store
}
