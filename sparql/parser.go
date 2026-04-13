package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Parse parses a SPARQL query string into a ParsedQuery.
// Only SELECT queries are supported.
func Parse(sparql string) (*ParsedQuery, error) {
	q := &ParsedQuery{
		Prefixes: make(map[string]string),
	}

	// Normalize whitespace
	s := normalizeWhitespace(sparql)

	// Extract and remove PREFIX declarations
	s = parsePrefixes(s, q.Prefixes)
	s = strings.TrimSpace(s)

	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "SELECT") {
		return nil, fmt.Errorf("only SELECT queries are supported")
	}

	return parseSelect(s, q)
}

func normalizeWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

func parsePrefixes(s string, prefixes map[string]string) string {
	re := regexp.MustCompile(`(?i)PREFIX\s+(\w+):\s*<([^>]+)>`)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		prefixes[m[1]] = m[2]
	}
	return re.ReplaceAllString(s, "")
}

func parseSelect(s string, q *ParsedQuery) (*ParsedQuery, error) {
	q.Type = QuerySelect

	// Parse aggregates first (e.g., (COUNT(?s) as ?count))
	parseAggregates(s, q)

	// Match: SELECT [DISTINCT] <vars> [WHERE|GROUP BY]
	// More permissive regex that captures everything between SELECT and WHERE/GROUP BY
	selectRe := regexp.MustCompile(`(?i)SELECT\s+(DISTINCT\s+)?(.+?)\s+(WHERE|GROUP BY)`)
	m := selectRe.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("invalid SELECT query: expected SELECT [DISTINCT] <variables> WHERE { ... }")
	}

	q.Distinct = strings.TrimSpace(m[1]) != ""

	// Parse variables (including aggregate aliases) from captured group
	varsStr := strings.TrimSpace(m[2])
	if varsStr == "*" {
		q.Variables = nil // determined from patterns later
	} else {
		varRe := regexp.MustCompile(`[\?\$](\w+)|\(.*?\) as ([\?\$]\w+)`)
		varMatches := varRe.FindAllStringSubmatch(varsStr, -1)
		if varMatches != nil {
			// Handle both regular variables and aggregate aliases
			for _, vm := range varMatches {
				if vm[1] != "" {
					q.Variables = append(q.Variables, vm[1])
				} else if vm[2] != "" {
					q.Variables = append(q.Variables, vm[2])
				}
			}
		} else {
			// Fallback: just extract all ?var patterns
			simpleVarRe := regexp.MustCompile(`[\?\$](\w+)`)
			simpleMatches := simpleVarRe.FindAllStringSubmatch(varsStr, -1)
			for _, vm := range simpleMatches {
				q.Variables = append(q.Variables, vm[1])
			}
		}
	}

	// Extract WHERE clause body (if present)
	whereBody, remaining, err := extractBraceBlock(s, "WHERE")
	if err != nil {
		// If no WHERE, try to find GROUP BY directly
		remaining = s
		whereBody = ""
	}

	// Parse GROUP BY from remaining text
	parseGroupBy(remaining, q)

	// Parse LIMIT and OFFSET from remaining text after WHERE block
	parseLimitOffset(remaining, q)

	// Parse WHERE body if present
	if whereBody != "" {
		if err := parseWhereBody(whereBody, q); err != nil {
			return nil, err
		}
	}

	// If SELECT *, collect variables from all patterns
	if q.Variables == nil {
		vars := collectVariablesFromPatterns(q.Where)
		for _, opt := range q.Optional {
			vars = append(vars, collectVariablesFromPatterns(opt)...)
		}
		q.Variables = dedupStrings(vars)
	}

	// Also add aggregate aliases to variables if not already present (strip the ? or $ prefix)
	for _, agg := range q.Aggregates {
		found := false
		aliasName := strings.TrimPrefix(agg.Alias, "?")
		aliasName = strings.TrimPrefix(aliasName, "$")
		for _, v := range q.Variables {
			if v == aliasName {
				found = true
				break
			}
		}
		if !found && agg.Alias != "" {
			q.Variables = append(q.Variables, aliasName)
		}
	}

	return q, nil
}

// extractBraceBlock finds a keyword followed by { ... } and extracts the body.
// Returns the body content, the remaining text after the closing brace, and any error.
func extractBraceBlock(s string, keyword string) (body string, remaining string, err error) {
	upper := strings.ToUpper(s)
	idx := strings.Index(upper, strings.ToUpper(keyword))
	if idx == -1 {
		return "", s, fmt.Errorf("missing %s clause", keyword)
	}

	after := s[idx+len(keyword):]
	after = strings.TrimSpace(after)
	if !strings.HasPrefix(after, "{") {
		return "", s, fmt.Errorf("expected '{' after %s", keyword)
	}

	depth := 0
	closeIdx := -1
	inString := false
	for i, c := range after {
		if c == '"' && (i == 0 || after[i-1] != '\\') {
			inString = !inString
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				closeIdx = i
				break
			}
		}
	}

	if closeIdx == -1 {
		return "", s, fmt.Errorf("unmatched '{' in %s clause", keyword)
	}

	body = strings.TrimSpace(after[1:closeIdx])
	remaining = strings.TrimSpace(after[closeIdx+1:])
	return body, remaining, nil
}

func parseWhereBody(body string, q *ParsedQuery) error {
	// Extract UNION blocks first (they have lowest precedence)
	remaining := body
	for {
		upper := strings.ToUpper(remaining)
		unionIdx := strings.Index(upper, "UNION")
		if unionIdx == -1 {
			break
		}

		// Find the { ... } block before UNION
		beforeUnion := remaining[:unionIdx]
		braceStart := strings.LastIndex(beforeUnion, "{")
		if braceStart == -1 {
			return fmt.Errorf("expected '{' before UNION")
		}
		braceEnd := strings.LastIndex(beforeUnion, "}")

		var leftPatterns []TriplePattern
		if braceEnd == -1 || braceEnd < braceStart {
			// No closing brace yet, this is the first pattern block
			inner := remaining[braceStart+1:]
			patterns, err := parseTriplePatterns(inner, q.Prefixes)
			if err != nil {
				return fmt.Errorf("in UNION left side: %w", err)
			}
			leftPatterns = patterns
			remaining = remaining[unionIdx:]
		} else {
			// Has closing brace
			inner := beforeUnion[braceStart+1 : braceEnd]
			patterns, err := parseTriplePatterns(inner, q.Prefixes)
			if err != nil {
				return fmt.Errorf("in UNION left side: %w", err)
			}
			leftPatterns = patterns
			remaining = remaining[braceEnd+1:]
		}

		// Find the { ... } block after UNION
		upper = strings.ToUpper(remaining)
		unionIdx = strings.Index(upper, "UNION")
		if unionIdx == -1 {
			return fmt.Errorf("expected another UNION or pattern after UNION")
		}

		afterUnion := remaining[unionIdx+4:]
		braceStart = strings.Index(afterUnion, "{")
		if braceStart == -1 {
			return fmt.Errorf("expected '{' after UNION")
		}

		// Find matching closing brace
		depth := 0
		closeIdx := -1
		for i, c := range afterUnion[braceStart:] {
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					closeIdx = braceStart + i
					break
				}
			}
		}

		if closeIdx == -1 {
			return fmt.Errorf("unmatched '{' in UNION right side")
		}

		inner := afterUnion[braceStart+1 : closeIdx]
		rightPatterns, err := parseTriplePatterns(inner, q.Prefixes)
		if err != nil {
			return fmt.Errorf("in UNION right side: %w", err)
		}

		// Add both patterns as a union group
		q.Union = append(q.Union, leftPatterns, rightPatterns)

		remaining = afterUnion[closeIdx+1:]
	}

	// Extract VALUES blocks
	remaining = parseValues(remaining, q)

	// Extract OPTIONAL blocks
	for {
		upper := strings.ToUpper(remaining)
		optIdx := strings.Index(upper, "OPTIONAL")
		if optIdx == -1 {
			break
		}

		// Extract the OPTIONAL block
		optBody, rest, err := extractBraceBlock(remaining[optIdx:], "OPTIONAL")
		if err != nil {
			return err
		}

		patterns, err := parseTriplePatterns(optBody, q.Prefixes)
		if err != nil {
			return fmt.Errorf("in OPTIONAL block: %w", err)
		}
		q.Optional = append(q.Optional, patterns)

		// Replace the OPTIONAL block in remaining with empty string
		remaining = remaining[:optIdx] + rest
	}

	// Extract FILTER clauses
	remaining = parseFilters(remaining, q)

	// Parse remaining triple patterns
	patterns, err := parseTriplePatterns(remaining, q.Prefixes)
	if err != nil {
		return err
	}
	q.Where = patterns

	return nil
}

func parseFilters(s string, q *ParsedQuery) string {
	remaining := s
	for {
		upper := strings.ToUpper(remaining)
		filterIdx := strings.Index(upper, "FILTER")
		if filterIdx == -1 {
			break
		}

		// Work with the raw (untrimmed) suffix to keep offsets aligned
		raw := remaining[filterIdx+6:]

		// Find the opening paren in the raw suffix
		openIdx := strings.IndexByte(raw, '(')
		if openIdx == -1 {
			break
		}

		// Find matching close paren
		depth := 0
		closeIdx := -1
		inStr := false
		for i := openIdx; i < len(raw); i++ {
			c := raw[i]
			if c == '"' && (i == 0 || raw[i-1] != '\\') {
				inStr = !inStr
			}
			if inStr {
				continue
			}
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 {
					closeIdx = i
					break
				}
			}
		}

		if closeIdx == -1 {
			break
		}

		filterContent := strings.TrimSpace(raw[openIdx+1 : closeIdx])
		filter, err := parseFilterExpr(filterContent)
		if err == nil {
			q.Filters = append(q.Filters, filter)
		}

		// Remove FILTER(...) from remaining: everything from filterIdx to filterIdx+6+closeIdx+1
		endPos := filterIdx + 6 + closeIdx + 1
		remaining = remaining[:filterIdx] + remaining[endPos:]
	}

	return remaining
}

func parseValues(s string, q *ParsedQuery) string {
	remaining := s
	for {
		upper := strings.ToUpper(remaining)
		valuesIdx := strings.Index(upper, "VALUES")
		if valuesIdx == -1 {
			break
		}

		raw := remaining[valuesIdx+6:]
		raw = strings.TrimSpace(raw)

		// Extract variables: (?var1 ?var2) or ?var
		varRe := regexp.MustCompile(`^\(([^)]+)\)`)
		m := varRe.FindStringSubmatch(raw)
		if m != nil {
			varsStr := m[1]
			varRe2 := regexp.MustCompile(`[\?\$](\w+)`)
			for _, vm := range varRe2.FindAllStringSubmatch(varsStr, -1) {
				q.Values.Variables = append(q.Values.Variables, vm[1])
			}
			raw = raw[len(m[0]):]
		} else {
			// Single variable
			varRe2 := regexp.MustCompile(`^[\?\$](\w+)`)
			if vm := varRe2.FindStringSubmatch(raw); vm != nil {
				q.Values.Variables = append(q.Values.Variables, vm[1])
				raw = raw[len(vm[0]):]
			} else {
				break
			}
		}

		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "{") {
			break
		}

		// Find the closing brace
		depth := 0
		closeIdx := -1
		for i, c := range raw {
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					closeIdx = i
					break
				}
			}
		}

		if closeIdx == -1 {
			break
		}

		valuesBody := raw[1:closeIdx]

		// Parse each value row
		for _, row := range strings.Split(valuesBody, ".") {
			row = strings.TrimSpace(row)
			if row == "" {
				continue
			}
			var values []string
			tokens := strings.Fields(row)
			for _, t := range tokens {
				values = append(values, expandTerm(t, q.Prefixes))
			}
			if len(values) > 0 {
				q.Values.Values = append(q.Values.Values, values)
			}
		}

		endPos := valuesIdx + 6 + closeIdx + 1
		if endPos < len(remaining) {
			remaining = remaining[:valuesIdx] + remaining[endPos:]
		} else {
			remaining = remaining[:valuesIdx]
		}
	}

	return remaining
}

func parseFilterExpr(content string) (Filter, error) {
	content = strings.TrimSpace(content)

	// Check for regex(...)
	regexRe := regexp.MustCompile(`(?i)regex\s*\(\s*([\?\$]\w+)\s*,\s*"([^"]+)"`)
	if m := regexRe.FindStringSubmatch(content); m != nil {
		return Filter{
			Op:    FilterRegex,
			Left:  m[1],
			Right: m[2],
		}, nil
	}

	// Check for != (before = to avoid ambiguity)
	neqRe := regexp.MustCompile(`([\?\$]\w+)\s*!=\s*(.+)`)
	if m := neqRe.FindStringSubmatch(content); m != nil {
		return Filter{
			Op:    FilterNotEquals,
			Left:  m[1],
			Right: cleanFilterValue(strings.TrimSpace(m[2])),
		}, nil
	}

	// Check for =
	eqRe := regexp.MustCompile(`([\?\$]\w+)\s*=\s*(.+)`)
	if m := eqRe.FindStringSubmatch(content); m != nil {
		return Filter{
			Op:    FilterEquals,
			Left:  m[1],
			Right: cleanFilterValue(strings.TrimSpace(m[2])),
		}, nil
	}

	return Filter{}, fmt.Errorf("unsupported FILTER expression: %s", content)
}

func cleanFilterValue(s string) string {
	// Remove surrounding quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func parseTriplePatterns(s string, prefixes map[string]string) ([]TriplePattern, error) {
	var patterns []TriplePattern

	statements := splitStatements(s)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		tokens := tokenize(stmt)
		if len(tokens) < 3 {
			return nil, fmt.Errorf("invalid triple pattern (need at least 3 terms): %q", stmt)
		}

		patterns = append(patterns, TriplePattern{
			Subject:   expandTerm(tokens[0], prefixes),
			Predicate: expandTerm(tokens[1], prefixes),
			Object:    expandTerm(tokens[2], prefixes),
		})
	}

	return patterns, nil
}

// splitStatements splits SPARQL statements by period, respecting URIs and string literals.
func splitStatements(s string) []string {
	var statements []string
	var current strings.Builder
	inURI := false
	inLiteral := false
	escape := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escape {
			current.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' {
			current.WriteByte(c)
			escape = true
			continue
		}
		if c == '<' && !inLiteral {
			inURI = true
			current.WriteByte(c)
			continue
		}
		if c == '>' && inURI {
			inURI = false
			current.WriteByte(c)
			continue
		}
		if c == '"' && !inURI {
			inLiteral = !inLiteral
			current.WriteByte(c)
			continue
		}
		if c == '.' && !inURI && !inLiteral {
			if i+1 >= len(s) || s[i+1] == ' ' || s[i+1] == '\t' || s[i+1] == '\n' {
				stmt := strings.TrimSpace(current.String())
				if stmt != "" {
					statements = append(statements, stmt)
				}
				current.Reset()
				continue
			}
		}
		current.WriteByte(c)
	}

	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// tokenize splits a SPARQL statement into tokens, preserving URIs and string literals.
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inURI := false
	inLiteral := false
	escape := false

	for _, c := range s {
		if escape {
			current.WriteRune(c)
			escape = false
			continue
		}
		if c == '\\' {
			current.WriteRune(c)
			escape = true
			continue
		}
		if c == '<' && !inLiteral {
			inURI = true
			current.WriteRune(c)
			continue
		}
		if c == '>' && inURI {
			inURI = false
			current.WriteRune(c)
			tokens = append(tokens, current.String())
			current.Reset()
			continue
		}
		if c == '"' {
			inLiteral = !inLiteral
			current.WriteRune(c)
			continue
		}
		if (c == ' ' || c == '\t') && !inURI && !inLiteral {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(c)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// expandTerm expands a prefixed term, URI in angle brackets, or leaves variables/literals unchanged.
func expandTerm(term string, prefixes map[string]string) string {
	// Variables
	if strings.HasPrefix(term, "?") || strings.HasPrefix(term, "$") {
		return term
	}

	// Full URI in angle brackets
	if strings.HasPrefix(term, "<") && strings.HasSuffix(term, ">") {
		return term[1 : len(term)-1]
	}

	// String literal
	if strings.HasPrefix(term, "\"") {
		// Strip quotes
		inner := term
		if len(inner) >= 2 && inner[len(inner)-1] == '"' {
			inner = inner[1 : len(inner)-1]
		}
		return inner
	}

	// rdf:type shorthand
	if term == "a" {
		return "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	}

	// Prefixed name (e.g., foaf:name)
	if idx := strings.Index(term, ":"); idx > 0 {
		prefix := term[:idx]
		local := term[idx+1:]
		if base, ok := prefixes[prefix]; ok {
			return base + local
		}
	}

	return term
}

// parseAggregates extracts aggregate expressions like (COUNT(?s) as ?count)
func parseAggregates(s string, q *ParsedQuery) {
	// Match: (COUNT(?var) as ?alias), (SUM(?var) as ?alias), etc.
	aggRe := regexp.MustCompile(`(?i)\((COUNT|SUM|MIN|MAX|AVG)\s*\(\s*([\?\$]\w+)\s*\)\s+as\s+([\?\$]\w+)\)`)
	matches := aggRe.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		agg := AggregateExpression{
			Function: strings.ToUpper(m[1]),
			Variable: m[2],
			Alias:    m[3],
		}
		q.Aggregates = append(q.Aggregates, agg)
	}

	// Also handle DISTINCT in aggregates: (COUNT(DISTINCT ?var) as ?alias)
	aggDistinctRe := regexp.MustCompile(`(?i)\((COUNT|SUM|MIN|MAX|AVG)\s*\(\s*DISTINCT\s+([\?\$]\w+)\s*\)\s+as\s+([\?\$]\w+)\)`)
	distinctMatches := aggDistinctRe.FindAllStringSubmatch(s, -1)
	for _, m := range distinctMatches {
		agg := AggregateExpression{
			Function: strings.ToUpper(m[1]),
			Variable: m[2],
			Alias:    m[3],
			Distinct: true,
		}
		q.Aggregates = append(q.Aggregates, agg)
	}
}

// parseGroupBy extracts GROUP BY variables
func parseGroupBy(s string, q *ParsedQuery) {
	groupByRe := regexp.MustCompile(`(?i)GROUP BY\s+(.+?)(?:LIMIT|OFFSET|$)`)
	m := groupByRe.FindStringSubmatch(s)
	if m != nil {
		groupByVars := strings.TrimSpace(m[1])
		varRe := regexp.MustCompile(`[\?\$](\w+)`)
		varMatches := varRe.FindAllStringSubmatch(groupByVars, -1)
		for _, vm := range varMatches {
			q.GroupBy = append(q.GroupBy, vm[1])
		}
	}
}

func parseLimitOffset(s string, q *ParsedQuery) {
	limitRe := regexp.MustCompile(`(?i)LIMIT\s+(\d+)`)
	if m := limitRe.FindStringSubmatch(s); m != nil {
		q.Limit, _ = strconv.Atoi(m[1])
	}

	offsetRe := regexp.MustCompile(`(?i)OFFSET\s+(\d+)`)
	if m := offsetRe.FindStringSubmatch(s); m != nil {
		q.Offset, _ = strconv.Atoi(m[1])
	}
}

func collectVariablesFromPatterns(patterns []TriplePattern) []string {
	var vars []string
	seen := make(map[string]bool)

	for _, p := range patterns {
		for _, term := range []string{p.Subject, p.Predicate, p.Object} {
			if isVariable(term) {
				name := term[1:]
				if !seen[name] {
					seen[name] = true
					vars = append(vars, name)
				}
			}
		}
	}

	return vars
}

func isVariable(s string) bool {
	return strings.HasPrefix(s, "?") || strings.HasPrefix(s, "$")
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
