// Package query provides a minimal SPARQL query engine for RDF triple stores.
//
// Supported SPARQL features:
//   - SELECT queries with WHERE clause containing triple patterns
//   - Variables: ?s, ?p, ?o style
//   - FILTER with string comparison (=, !=) and regex
//   - OPTIONAL triple patterns
//   - PREFIX declarations
//   - LIMIT and OFFSET
//
// Not supported: CONSTRUCT, ASK, DESCRIBE, aggregation, subqueries,
// UNION, property paths, BIND, VALUES.
package query

// QueryType represents the type of SPARQL query.
type QueryType int

const (
	QuerySelect QueryType = iota
)

// AggregateExpression represents an aggregate function like COUNT, SUM, etc.
type AggregateExpression struct {
	Function string // COUNT, SUM, MIN, MAX, AVG
	Variable string // the variable to aggregate
	Alias    string // the alias for the result (e.g., ?count)
	Distinct bool
}

// ParsedQuery represents a parsed SPARQL query.
type ParsedQuery struct {
	Type       QueryType
	Variables  []string // projected variables (nil = SELECT *)
	Distinct   bool
	Where      []TriplePattern
	Optional   [][]TriplePattern // each inner slice is an OPTIONAL group
	Filters    []Filter
	Prefixes   map[string]string
	Limit      int // 0 = no limit
	Offset     int // 0 = no offset
	Aggregates []AggregateExpression
	GroupBy    []string
}

// TriplePattern represents a single triple pattern in a WHERE clause.
// Values starting with ? or $ are variables.
type TriplePattern struct {
	Subject   string
	Predicate string
	Object    string
}

// FilterOp represents a FILTER operation type.
type FilterOp int

const (
	FilterEquals    FilterOp = iota // ?x = "value"
	FilterNotEquals                 // ?x != "value"
	FilterRegex                     // regex(?x, "pattern")
)

// Filter represents a FILTER condition in a WHERE clause.
type Filter struct {
	Op    FilterOp
	Left  string // variable or value
	Right string // value or regex pattern
}
