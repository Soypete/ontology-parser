// Package query provides SPARQL query execution against a triple store.
//
// This package implements a SPARQL query engine that executes queries against
// a store.Store implementation. It supports SELECT queries with variable bindings,
// pattern matching, and filter expressions. Results are returned as variable
// bindings (map of variable names to values) and/or matching triples.
//
// The query engine parses SPARQL strings into an internal query representation
// using the package's Parse function, then executes against the provided store.
// It's designed to work with any implementation of the store.Store interface.
//
// Example:
//
//	engine := sparql.NewEngine(store)
//	result, err := engine.Execute("SELECT ?s ?p ?o WHERE { ?s ?p ?o }")
//	for _, binding := range result.Bindings {
//	    fmt.Println(binding["s"], binding["p"], binding["o"])
//	}
package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/soypete/ontology-go/store"
	"github.com/soypete/ontology-go/types"
)

// Engine executes SPARQL queries against a triple store.
type Engine struct {
	store       store.Store
	skosOptions SKOSOptions
}

// NewEngine creates a new SPARQL query engine.
func NewEngine(s store.Store, opts ...EngineOption) *Engine {
	e := &Engine{store: s, skosOptions: defaultSKOSOptions}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ApplyOption applies an engine option to the engine.
func (e *Engine) ApplyOption(opt EngineOption) {
	opt(e)
}

// Execute parses and executes a SPARQL query string, returning a QueryResult.
func (e *Engine) Execute(sparql string) (*types.QueryResult, error) {
	q, err := Parse(sparql)
	if err != nil {
		return nil, fmt.Errorf("sparql parse error: %w", err)
	}

	return e.ExecuteParsed(q)
}

// ExecuteParsed executes a pre-parsed query against the store.
func (e *Engine) ExecuteParsed(q *ParsedQuery) (*types.QueryResult, error) {
	if q.Type != QuerySelect {
		return nil, fmt.Errorf("only SELECT queries are supported")
	}

	// Get all triples first - SKOS inference needs all triples to generate correct inferences
	allTriples := e.store.All()

	// Apply SKOS inference if enabled (must run on ALL triples, not filtered)
	allTriples = e.inferSKOSTriples(allTriples)

	// Now optimize by pre-filtering the inferred triples
	allTriples = e.getOptimizedTriplesWithOptional(q.Where, q.Optional, allTriples)

	// Match the basic graph pattern
	bindings, matchedTriples, path := matchBGP(q.Where, allTriples)

	// Apply VALUES clause (filter bindings to only include specified values)
	if len(q.Values.Values) > 0 {
		bindings = applyValues(bindings, q.Values)
	}

	// Apply UNION patterns
	if len(q.Union) > 0 {
		unionBindings, unionTriples, unionPath := matchUnion(q.Union, allTriples)
		// Merge UNION results with regular bindings
		bindings = append(bindings, unionBindings...)
		matchedTriples = append(matchedTriples, unionTriples...)
		path = append(path, unionPath...)
	}

	// Apply OPTIONAL patterns
	for _, optPatterns := range q.Optional {
		bindings = applyOptional(bindings, optPatterns, allTriples)
	}

	// Apply FILTERs
	for _, f := range q.Filters {
		bindings = applyFilter(bindings, f)
	}

	// Apply GROUP BY
	if len(q.GroupBy) > 0 {
		bindings = applyGroupBy(bindings, q.GroupBy)
	}

	// Apply aggregates
	if len(q.Aggregates) > 0 {
		bindings = applyAggregates(bindings, q.Aggregates, q.GroupBy, q.Distinct)
	}

	// Apply DISTINCT (after aggregates)
	if q.Distinct && len(q.Aggregates) == 0 {
		bindings = distinct(bindings, q.Variables)
	}

	// Apply OFFSET
	if q.Offset > 0 && q.Offset < len(bindings) {
		bindings = bindings[q.Offset:]
	} else if q.Offset >= len(bindings) {
		bindings = nil
	}

	// Apply LIMIT
	if q.Limit > 0 && len(bindings) > q.Limit {
		bindings = bindings[:q.Limit]
	}

	// Project to selected variables
	projected := make([]map[string]string, 0, len(bindings))
	for _, binding := range bindings {
		row := make(map[string]string)
		for _, v := range q.Variables {
			if val, ok := binding[v]; ok {
				row[v] = val
			}
		}
		projected = append(projected, row)
	}

	return &types.QueryResult{
		Bindings: projected,
		Triples:  matchedTriples,
		Path:     path,
	}, nil
}

// getOptimizedTriplesWithOptional filters triples based on fixed values in patterns.
// Takes a pre-filtered set of triples (after SKOS inference) and further filters by patterns.
func (e *Engine) getOptimizedTriplesWithOptional(patterns []TriplePattern, optionals [][]TriplePattern, triples []types.Triple) []types.Triple {
	// Collect all patterns including OPTIONALs
	allPatterns := make([]TriplePattern, len(patterns))
	copy(allPatterns, patterns)
	for _, optGroup := range optionals {
		allPatterns = append(allPatterns, optGroup...)
	}

	if len(allPatterns) == 0 {
		return triples
	}

	// For multi-pattern queries, only optimize if ALL patterns share the same fixed predicate
	// (otherwise it's a join across different predicates and we need all triples)
	if len(allPatterns) > 1 {
		var commonPredicate string
		for _, p := range allPatterns {
			if isVariable(p.Predicate) {
				return triples // Variable predicate - can't optimize
			}
			if commonPredicate == "" {
				commonPredicate = p.Predicate
			} else if commonPredicate != p.Predicate {
				return triples // Different predicates - can't optimize
			}
		}
		// All patterns share the same fixed predicate - filter by predicate
		if commonPredicate != "" {
			return filterTriplesByPredicate(triples, commonPredicate)
		}
	}

	// Single pattern query - convert variables to empty string for filtering
	p := allPatterns[0]
	subject := ""
	predicate := ""
	object := ""

	if !isVariable(p.Subject) {
		subject = p.Subject
	}
	if !isVariable(p.Predicate) {
		predicate = p.Predicate
	}
	if !isVariable(p.Object) {
		object = p.Object
	}

	// Only optimize if we have at least one fixed value
	if subject != "" || predicate != "" || object != "" {
		return filterTriples(triples, subject, predicate, object)
	}

	return triples
}

func filterTriples(triples []types.Triple, subject, predicate, object string) []types.Triple {
	var result []types.Triple
	for _, t := range triples {
		if (subject == "" || t.Subject == subject) &&
			(predicate == "" || t.Predicate == predicate) &&
			(object == "" || t.Object == object) {
			result = append(result, t)
		}
	}
	return result
}

func filterTriplesByPredicate(triples []types.Triple, predicate string) []types.Triple {
	var result []types.Triple
	for _, t := range triples {
		if t.Predicate == predicate {
			result = append(result, t)
		}
	}
	return result
}

// matchBGP evaluates a basic graph pattern against triples.
// Returns bindings, matched triples, and the traversal path.
func matchBGP(patterns []TriplePattern, triples []types.Triple) ([]map[string]string, []types.Triple, []string) {
	if len(patterns) == 0 {
		return []map[string]string{{}}, nil, nil
	}

	bindings := []map[string]string{{}}
	var matchedTriples []types.Triple
	var path []string
	seen := make(map[string]bool) // deduplicate matched triples

	for _, pattern := range patterns {
		var newBindings []map[string]string

		for _, binding := range bindings {
			for _, triple := range triples {
				newBinding := tryMatch(pattern, triple, binding)
				if newBinding != nil {
					newBindings = append(newBindings, newBinding)

					// Track matched triple
					key := triple.Subject + "|" + triple.Predicate + "|" + triple.Object
					if !seen[key] {
						seen[key] = true
						matchedTriples = append(matchedTriples, triple)
						path = append(path, triple.Subject, triple.Predicate, triple.Object)
					}
				}
			}
		}

		bindings = newBindings
		if len(bindings) == 0 {
			break
		}
	}

	return bindings, matchedTriples, path
}

// applyOptional adds bindings from OPTIONAL patterns.
// Existing bindings that don't match are kept with unbound optional variables.
func applyOptional(bindings []map[string]string, patterns []TriplePattern, triples []types.Triple) []map[string]string {
	var result []map[string]string

	for _, binding := range bindings {
		optBindings := []map[string]string{copyBinding(binding)}

		matched := false
		for _, pattern := range patterns {
			var newBindings []map[string]string
			for _, b := range optBindings {
				for _, triple := range triples {
					nb := tryMatch(pattern, triple, b)
					if nb != nil {
						newBindings = append(newBindings, nb)
						matched = true
					}
				}
			}
			if len(newBindings) > 0 {
				optBindings = newBindings
			}
		}

		if matched {
			result = append(result, optBindings...)
		} else {
			// OPTIONAL didn't match — keep original binding
			result = append(result, binding)
		}
	}

	return result
}

// tryMatch attempts to match a triple pattern against a triple, extending the binding.
// Returns nil if the match fails.
func tryMatch(pattern TriplePattern, triple types.Triple, binding map[string]string) map[string]string {
	nb := copyBinding(binding)

	if !matchTerm(pattern.Subject, triple.Subject, nb) {
		return nil
	}
	if !matchPredicate(pattern.Predicate, triple.Predicate, nb) {
		return nil
	}
	if !matchTerm(pattern.Object, triple.Object, nb) {
		return nil
	}

	return nb
}

// matchPredicate matches a pattern predicate against a triple predicate.
// Handles fx:anySlot magic property that matches any rdf:_N container membership property.
func matchPredicate(patternPred, triplePred string, binding map[string]string) bool {
	// Check for fx:anySlot magic property
	if isMagicProperty(patternPred, "anySlot") {
		// Match any rdf:_N property
		return isContainerMembershipProperty(triplePred)
	}

	// fx:cardinal(?a) - returns true if ?a is a container membership property
	if isMagicFunction(patternPred, "cardinal") {
		// In filter context, check if the bound value is a container membership property
		if varName := extractVariableFromMagicFunc(patternPred); varName != "" {
			if boundVal, ok := binding[varName]; ok {
				return isContainerMembershipProperty(boundVal)
			}
		}
		return false
	}

	// Standard term matching
	return matchTerm(patternPred, triplePred, binding)
}

// isMagicProperty checks if a term is a magic property like fx:anySlot.
func isMagicProperty(term, propName string) bool {
	return strings.HasPrefix(term, "http://sparql.xyz/facade-x/ns/") &&
		strings.HasSuffix(term, propName)
}

// isMagicFunction checks if a term is a magic function like fx:cardinal.
func isMagicFunction(term, funcName string) bool {
	return strings.HasPrefix(term, "http://sparql.xyz/facade-x/ns/") &&
		strings.HasSuffix(term, funcName)
}

// extractVariableFromMagicFunc extracts the variable name from a magic function term.
func extractVariableFromMagicFunc(term string) string {
	re := regexp.MustCompile(`([\?\$]\w+)`)
	m := re.FindStringSubmatch(term)
	if m != nil {
		return m[1][1:] // Remove ? or $
	}
	return ""
}

// isContainerMembershipProperty returns true if the predicate is rdf:_N (container membership).
func isContainerMembershipProperty(pred string) bool {
	return strings.HasPrefix(pred, "http://www.w3.org/1999/02/22-rdf-syntax-ns#_")
}

// matchTerm matches a pattern term against a concrete value, updating bindings.
func matchTerm(term, value string, binding map[string]string) bool {
	if isVariable(term) {
		varName := term[1:]
		if existing, ok := binding[varName]; ok {
			return existing == value
		}
		binding[varName] = value
		return true
	}
	return term == value
}

// applyFilter filters bindings based on a filter condition.
func applyFilter(bindings []map[string]string, f Filter) []map[string]string {
	var result []map[string]string
	for _, binding := range bindings {
		if evaluateFilter(f, binding) {
			result = append(result, binding)
		}
	}
	return result
}

func evaluateFilter(f Filter, binding map[string]string) bool {
	switch f.Op {
	case FilterEquals:
		left := resolveValue(f.Left, binding)
		right := f.Right
		return left == right
	case FilterNotEquals:
		left := resolveValue(f.Left, binding)
		right := f.Right
		return left != right
	case FilterRegex:
		left := resolveValue(f.Left, binding)
		matched, _ := regexp.MatchString(f.Right, left)
		return matched
	case FilterFunction:
		return evaluateFilterFunction(f, binding)
	default:
		return true
	}
}

func evaluateFilterFunction(f Filter, binding map[string]string) bool {
	if len(f.Args) == 0 {
		return true
	}

	// Resolve first argument (usually a variable)
	arg0 := resolveValue(f.Args[0], binding)

	switch f.Func {
	case FuncContains:
		if len(f.Args) >= 2 {
			return strings.Contains(arg0, f.Args[1])
		}
	case FuncStrStarts, FuncStartsWith:
		if len(f.Args) >= 2 {
			return strings.HasPrefix(arg0, f.Args[1])
		}
	case FuncStrEnds, FuncEndsWith:
		if len(f.Args) >= 2 {
			return strings.HasSuffix(arg0, f.Args[1])
		}
	case FuncLcase:
		// Lcase in filter context - compare lowercase
		if len(f.Args) >= 2 {
			return strings.ToLower(arg0) == strings.ToLower(f.Args[1])
		}
	case FuncUcase:
		// Ucase in filter context - compare uppercase
		if len(f.Args) >= 2 {
			return strings.ToUpper(arg0) == strings.ToUpper(f.Args[1])
		}
	case FuncReplace:
		if len(f.Args) >= 3 {
			return strings.ReplaceAll(arg0, f.Args[1], f.Args[2]) != arg0
		}
	case FuncIsURI:
		if len(f.Args) >= 1 {
			val := arg0
			return strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") || strings.HasPrefix(val, "urn:")
		}
	case FuncIsLiteral:
		if len(f.Args) >= 1 {
			val := arg0
			// Not a URI means it's likely a literal
			return !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") && !strings.HasPrefix(val, "urn:")
		}
	case FuncStr:
		// str() function returns the string representation
		// Used in filter to compare strings
		if len(f.Args) >= 2 {
			return arg0 == f.Args[1]
		}
		return arg0 != ""
	case FuncSubstr:
		// In FILTER context, check if substring from position exists
		if len(f.Args) >= 2 {
			start, _ := strconv.Atoi(f.Args[1])
			if start < len(arg0) {
				return true // substring from position exists
			}
			return false
		}
	case FuncStrLen:
		return len(arg0) > 0
	case FuncYear, FuncMonth, FuncDay, FuncHours, FuncMinutes, FuncSeconds:
		// Date extraction - basic check
		return arg0 != ""
	}

	return true
}

func resolveValue(term string, binding map[string]string) string {
	if isVariable(term) {
		return binding[term[1:]]
	}
	// Strip quotes from string literals
	if strings.HasPrefix(term, "\"") && strings.HasSuffix(term, "\"") {
		return term[1 : len(term)-1]
	}
	return term
}

// distinct removes duplicate result rows based on the projected variables.
func distinct(bindings []map[string]string, variables []string) []map[string]string {
	seen := make(map[string]bool)
	var result []map[string]string

	for _, binding := range bindings {
		var parts []string
		for _, v := range variables {
			parts = append(parts, binding[v])
		}
		key := strings.Join(parts, "\x00")
		if !seen[key] {
			seen[key] = true
			result = append(result, binding)
		}
	}

	return result
}

func copyBinding(b map[string]string) map[string]string {
	nb := make(map[string]string, len(b))
	for k, v := range b {
		nb[k] = v
	}
	return nb
}

func applyGroupBy(bindings []map[string]string, groupByVars []string) []map[string]string {
	if len(groupByVars) == 0 {
		return bindings
	}

	groups := make(map[string][]map[string]string)
	for _, binding := range bindings {
		key := makeKey(binding, groupByVars)
		groups[key] = append(groups[key], binding)
	}

	var result []map[string]string
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		result = append(result, group...)
	}
	return result
}

func makeKey(binding map[string]string, vars []string) string {
	var parts []string
	for _, v := range vars {
		parts = append(parts, binding[v])
	}
	return strings.Join(parts, "\x00")
}

func mergeBindingGroup(group []map[string]string, groupByVars []string) map[string]string {
	merged := make(map[string]string)
	for _, g := range group {
		for k, v := range g {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
	}
	return merged
}

func applyAggregates(bindings []map[string]string, aggregates []AggregateExpression, groupByVars []string, distinct bool) []map[string]string {
	if len(aggregates) == 0 {
		return bindings
	}

	if len(groupByVars) == 0 {
		var result []map[string]string
		merged := mergeBindingGroup(bindings, nil)
		for _, agg := range aggregates {
			aliasName := strings.TrimPrefix(agg.Alias, "?")
			aliasName = strings.TrimPrefix(aliasName, "$")
			merged[aliasName] = computeAggregate(bindings, agg)
		}
		result = append(result, merged)
		return result
	}

	groups := make(map[string][]map[string]string)
	for _, binding := range bindings {
		key := makeKey(binding, groupByVars)
		groups[key] = append(groups[key], binding)
	}

	var result []map[string]string
	for _, group := range groups {
		merged := mergeBindingGroup(group, groupByVars)
		for _, agg := range aggregates {
			aliasName := strings.TrimPrefix(agg.Alias, "?")
			aliasName = strings.TrimPrefix(aliasName, "$")
			merged[aliasName] = computeAggregate(group, agg)
		}
		result = append(result, merged)
	}
	return result
}

func computeAggregate(group []map[string]string, agg AggregateExpression) string {
	var values []string
	var numericValues []float64

	// Debug: check what variable we're looking for
	varKey := strings.TrimPrefix(agg.Variable, "?")
	varKey = strings.TrimPrefix(varKey, "$")

	for _, binding := range group {
		if val, ok := binding[varKey]; ok {
			values = append(values, val)
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				numericValues = append(numericValues, f)
			}
		}
	}

	if agg.Distinct {
		seen := make(map[string]bool)
		var uniqueValues []string
		for _, v := range values {
			if !seen[v] {
				seen[v] = true
				uniqueValues = append(uniqueValues, v)
			}
		}
		values = uniqueValues
		numericValues = nil
		for _, v := range values {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				numericValues = append(numericValues, f)
			}
		}
	}

	switch agg.Function {
	case "COUNT":
		return strconv.Itoa(len(values))
	case "SUM":
		if len(numericValues) == 0 {
			return "0"
		}
		var sum float64
		for _, v := range numericValues {
			sum += v
		}
		return strconv.FormatFloat(sum, 'f', -1, 64)
	case "MIN":
		if len(numericValues) == 0 {
			if len(values) > 0 {
				return values[0]
			}
			return ""
		}
		min := numericValues[0]
		for _, v := range numericValues {
			if v < min {
				min = v
			}
		}
		return strconv.FormatFloat(min, 'f', -1, 64)
	case "MAX":
		if len(numericValues) == 0 {
			if len(values) > 0 {
				return values[len(values)-1]
			}
			return ""
		}
		max := numericValues[0]
		for _, v := range numericValues {
			if v > max {
				max = v
			}
		}
		return strconv.FormatFloat(max, 'f', -1, 64)
	case "AVG":
		if len(numericValues) == 0 {
			return "0"
		}
		sum := 0.0
		for _, v := range numericValues {
			sum += v
		}
		return strconv.FormatFloat(sum/float64(len(numericValues)), 'f', -1, 64)
	default:
		return ""
	}
}

func applyValues(bindings []map[string]string, values ValuesClause) []map[string]string {
	if len(values.Values) == 0 || len(values.Variables) == 0 {
		return bindings
	}

	var result []map[string]string
	for _, binding := range bindings {
		for _, valueRow := range values.Values {
			match := true
			for varIdx, varName := range values.Variables {
				if varIdx >= len(valueRow) {
					continue
				}
				if existingVal, ok := binding[varName]; ok {
					if existingVal != valueRow[varIdx] {
						match = false
						break
					}
				} else {
					// Variable not bound, we can bind it
					newBinding := copyBinding(binding)
					newBinding[varName] = valueRow[varIdx]
					result = append(result, newBinding)
					match = false // don't add original
				}
			}
			if match && len(values.Variables) == len(valueRow) {
				allBound := true
				for _, varName := range values.Variables {
					if _, ok := binding[varName]; !ok {
						allBound = false
						break
					}
				}
				if allBound {
					result = append(result, binding)
				}
			}
		}
	}
	return result
}

func matchUnion(unionPatterns [][]TriplePattern, triples []types.Triple) ([]map[string]string, []types.Triple, []string) {
	var allBindings []map[string]string
	var allTriples []types.Triple
	var path []string

	seenTriples := make(map[string]bool)

	for _, patterns := range unionPatterns {
		bindings, matched, p := matchBGP(patterns, triples)
		allBindings = append(allBindings, bindings...)
		for _, t := range matched {
			key := t.Subject + "|" + t.Predicate + "|" + t.Object
			if !seenTriples[key] {
				seenTriples[key] = true
				allTriples = append(allTriples, t)
			}
		}
		path = append(path, p...)
	}

	return allBindings, allTriples, path
}
