// Package store provides interfaces and implementations for RDF triple storage
// organized by named graphs.
//
// This package defines the Store interface for RDF triple persistence and provides
// a MemoryStore implementation for in-memory storage. Named graphs allow organizing
// triples into separate contexts, enabling use cases like TBOX/ABOX separation
// and dataset versioning.
//
// The Store interface supports:
//   - Register: Add triples to a named graph
//   - Remove: Delete an entire graph
//   - List: Enumerate all graphs
//   - All: Retrieve all triples across graphs
//   - Match: Pattern-match triples by subject/predicate/object
//
// Example:
//
//	store := store.NewMemoryStore()
//	store.Register("products", []types.Triple{
//	    {Subject: "https://example.org/product/1", Predicate: "rdf:type", Object: "schema:Product"},
//	})
//	triples := store.Match("", "rdf:type", "schema:Product")
package store

import (
	"fmt"
	"sync"

	"github.com/soypete/ontology-go/types"
)

// Store defines the interface for triple storage with named graphs.
type Store interface {
	// Register adds triples under a named graph.
	Register(name string, triples []types.Triple) error

	// Remove removes a named graph and all its triples.
	Remove(name string) error

	// List returns the names of all registered graphs.
	List() []string

	// All returns all triples across all graphs.
	All() []types.Triple

	// Match returns triples matching the given pattern.
	// Empty string acts as a wildcard for that position.
	Match(subject, predicate, object string) []types.Triple
}

// MemoryStore is a thread-safe in-memory implementation of Store.
// Triples are organized by named graph with indices for efficient matching.
type MemoryStore struct {
	mu     sync.RWMutex
	graphs map[string][]types.Triple

	// Indices for fast lookup across all graphs
	bySubject   map[string][]int
	byPredicate map[string][]int
	byObject    map[string][]int
	allTriples  []types.Triple
}

// NewMemoryStore creates a new in-memory triple store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		graphs:      make(map[string][]types.Triple),
		bySubject:   make(map[string][]int),
		byPredicate: make(map[string][]int),
		byObject:    make(map[string][]int),
	}
}

// Register adds triples under a named graph. If the graph already exists,
// it is replaced.
func (m *MemoryStore) Register(name string, triples []types.Triple) error {
	if name == "" {
		return fmt.Errorf("graph name must not be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove existing graph if present
	if _, exists := m.graphs[name]; exists {
		m.removeGraphLocked(name)
	}

	// Set graph name on all triples
	stored := make([]types.Triple, len(triples))
	for i, t := range triples {
		t.Graph = name
		stored[i] = t
	}

	m.graphs[name] = stored

	// Rebuild indices
	m.rebuildIndices()

	return nil
}

// Remove removes a named graph and all its triples.
func (m *MemoryStore) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.graphs[name]; !exists {
		return fmt.Errorf("graph %q not found", name)
	}

	m.removeGraphLocked(name)
	m.rebuildIndices()
	return nil
}

// removeGraphLocked removes a graph without acquiring the lock.
func (m *MemoryStore) removeGraphLocked(name string) {
	delete(m.graphs, name)
}

// List returns the names of all registered graphs.
func (m *MemoryStore) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.graphs))
	for name := range m.graphs {
		names = append(names, name)
	}
	return names
}

// All returns all triples across all graphs.
func (m *MemoryStore) All() []types.Triple {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]types.Triple, len(m.allTriples))
	copy(result, m.allTriples)
	return result
}

// Match returns triples matching the given pattern.
// Empty string acts as a wildcard for that position.
func (m *MemoryStore) Match(subject, predicate, object string) []types.Triple {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use the most selective index
	var candidates []int

	if subject != "" {
		candidates = m.bySubject[subject]
	} else if predicate != "" {
		candidates = m.byPredicate[predicate]
	} else if object != "" {
		candidates = m.byObject[object]
	} else {
		// No filter — return all
		result := make([]types.Triple, len(m.allTriples))
		copy(result, m.allTriples)
		return result
	}

	var result []types.Triple
	for _, idx := range candidates {
		t := m.allTriples[idx]
		if (subject == "" || t.Subject == subject) &&
			(predicate == "" || t.Predicate == predicate) &&
			(object == "" || t.Object == object) {
			result = append(result, t)
		}
	}

	return result
}

// rebuildIndices rebuilds the flat allTriples slice and all indices.
func (m *MemoryStore) rebuildIndices() {
	m.allTriples = nil
	m.bySubject = make(map[string][]int)
	m.byPredicate = make(map[string][]int)
	m.byObject = make(map[string][]int)

	for _, triples := range m.graphs {
		for _, t := range triples {
			idx := len(m.allTriples)
			m.allTriples = append(m.allTriples, t)
			m.bySubject[t.Subject] = append(m.bySubject[t.Subject], idx)
			m.byPredicate[t.Predicate] = append(m.byPredicate[t.Predicate], idx)
			m.byObject[t.Object] = append(m.byObject[t.Object], idx)
		}
	}
}
