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

// RDF version constants for version detection and content negotiation.
const (
	// RDFVersion12 represents RDF 1.2 as defined by W3C CR dated 07 April 2026.
	RDFVersion12 = "1.2"
	// RDFVersion11 represents RDF 1.1 as defined by W3C Recommendation.
	RDFVersion11 = "1.1"
)

const (
	RDFNS         = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFType       = RDFNS + "type"
	RDFFirst      = RDFNS + "first"
	RDFRest       = RDFNS + "rest"
	RDFNil        = RDFNS + "nil"
	RDFLangString = RDFNS + "langString"

	// RDF 1.2 specific vocabulary
	RDFDirLangString = RDFNS + "dirLangString"
	RDFReifies       = RDFNS + "reifies"
	RDFSubject       = RDFNS + "subject"
	RDFPredicate     = RDFNS + "predicate"
	RDFObject        = RDFNS + "object"
)

const (
	// XSD namespace for XML Schema datatypes.
	XSD         = "http://www.w3.org/2001/XMLSchema#"
	XSDString   = XSD + "string"
	XSDInteger  = XSD + "integer"
	XSDDecimal  = XSD + "decimal"
	XSDDouble   = XSD + "double"
	XSDBoolean  = XSD + "boolean"
	XSDDate     = XSD + "date"
	XSDDateTime = XSD + "dateTime"
)

// Triple represents an RDF triple with an optional named graph.
// All fields support RDF 1.2 features including directional language tags
// and triple terms for reification.
type Triple struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
	Graph     string `json:"graph,omitempty"`

	// IsLiteral indicates whether the Object is an RDF literal (as opposed to an IRI or blank node).
	IsLiteral bool `json:"is_literal,omitempty"`

	// Datatype holds the datatype IRI for typed literals (e.g., "http://www.w3.org/2001/XMLSchema#string").
	// For language-tagged literals, this is typically rdf:langString or rdf:dirLangString (RDF 1.2).
	Datatype string `json:"datatype,omitempty"`

	// Language holds the language tag for language-tagged literals (e.g., "en", "en-US").
	// For RDF 1.2 directional language-tagged strings, this contains the language tag without direction.
	Language string `json:"language,omitempty"`

	// Direction holds the text direction for RDF 1.2 directional language-tagged strings.
	// Valid values are "ltr" (left-to-right) or "rtl" (right-to-left).
	// Empty when no direction is specified.
	Direction string `json:"direction,omitempty"`

	// IsTripleTerm indicates whether the Object is an RDF 1.2 triple term.
	// Triple terms allow nesting triples as objects: <<(s p o)>>
	IsTripleTerm bool `json:"is_triple_term,omitempty"`

	// Reifier holds the IRI or blank node identifier that reifies this triple.
	// Used in RDF 1.2 reified triple syntax: <<s p o ~reifier>>
	Reifier string `json:"reifier,omitempty"`
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
