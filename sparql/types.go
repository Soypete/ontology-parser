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
	Union      [][]TriplePattern // UNION blocks (each inner slice is an alternative)
	Values     ValuesClause
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
	FilterFunction                  // filter function call
)

// FilterFunctionType represents the type of SPARQL filter function.
type FilterFunctionType int

const (
	FuncContains FilterFunctionType = iota
	FuncStartsWith
	FuncEndsWith
	FuncLcase
	FuncUcase
	FuncReplace
	FuncStr
	FuncLang
	FuncDatatype
	FuncIsURI
	FuncIsLiteral
	FuncIsBlank
	FuncStrStarts
	FuncStrEnds
	FuncStrContains
	FuncSubstr
	FuncStrLen
	FuncConcat
	FuncYear
	FuncMonth
	FuncDay
	FuncHours
	FuncMinutes
	FuncSeconds
)

// Filter represents a FILTER condition in a WHERE clause.
type Filter struct {
	Op       FilterOp
	Left     string             // variable or value (for =, !=)
	Right    string             // value or regex pattern (for =, !=)
	Func     FilterFunctionType // function type (for FilterFunction)
	Args     []string           // function arguments
	FuncName string             // original function name for debugging
}

// GraphPattern represents a graph pattern which can be either basic triples or a UNION group.
type GraphPattern interface {
	isGraphPattern()
}

// BasicPattern represents a basic graph pattern (triple patterns).
type BasicPattern []TriplePattern

func (BasicPattern) isGraphPattern() {}

// UnionGroup represents a UNION of multiple graph patterns.
type UnionGroup struct {
	Patterns []GraphPattern
}

func (UnionGroup) isGraphPattern() {}

// PropertyPathType represents the type of property path.
type PropertyPathType int

const (
	PathZeroOrMore PropertyPathType = iota // *
	PathOneOrMore                          // +
	PathZeroOrOne                          // ?
)

// PropertyPath represents a property path expression in SPARQL.
type PropertyPath struct {
	PathType  PropertyPathType
	Predicate string
}

// ValuesClause represents a VALUES clause in SPARQL.
type ValuesClause struct {
	Variables []string
	Values    [][]string // each inner slice is a row of values
}
