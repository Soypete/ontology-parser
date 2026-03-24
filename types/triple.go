// Package types provides core RDF types and constants for the ontology-parser
// library.
//
// This package defines the fundamental data structures used across all packages,
// including Triple for RDF statements, Resource for fetched URI content, and
// QueryResult for SPARQL query outputs. It also provides RDF vocabulary
// constants like RDFNS and RDFType for common predicates.
//
// The types are designed to be serializable (JSON tags) and compatible with
// standard RDF data models. The Triple struct supports named graphs via the
// Graph field, enabling dataset-level organization.
//
// Example:
//
//	triple := types.Triple{
//	    Subject:   "https://example.org/person/1",
//	    Predicate: "rdf:type",
//	    Object:    "schema:Person",
//	    Graph:     "https://example.org/data",
//	}
package types

const (
	RDFNS   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFType = RDFNS + "type"
)

// Triple represents an RDF triple with an optional named graph.
type Triple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
	Graph     string `json:"graph,omitempty"` // name of the registered RDF source
}

// Resource represents a fetched URI resource.
type Resource struct {
	URI         string `json:"uri"`
	ContentType string `json:"content_type,omitempty"`
	Body        []byte `json:"body,omitempty"`
	Error       string `json:"error,omitempty"` // non-empty if fetch failed
}

// QueryResult holds the output of a SPARQL query execution.
type QueryResult struct {
	// Bindings contains SPARQL variable bindings per result row.
	Bindings []map[string]string `json:"bindings"`

	// Triples contains all triples that matched the query patterns.
	Triples []Triple `json:"triples"`

	// Path records the chain of subject→predicate→object traversals
	// taken during matching.
	Path []string `json:"path,omitempty"`

	// Resources holds fetched URL payloads, keyed by URI.
	// Only populated when fetch is enabled on the query.
	Resources map[string]Resource `json:"resources,omitempty"`
}
