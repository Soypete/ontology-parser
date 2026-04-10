// Package ttl provides W3C Turtle format parsing functionality.
//
// This package implements a parser for the W3C Turtle (TTL) RDF serialization
// format. Turtle is a compact, human-readable syntax for representing RDF graphs.
//
// Supported features:
//   - @prefix declarations for prefixed names
//   - @base for relative IRI resolution
//   - Full IRIs in angle brackets <...>
//   - Prefixed names (e.g., rdf:type, schema:Person)
//   - Predicate lists (;) for sharing subjects
//   - Object lists (,) for sharing predicates
//   - 'a' shorthand for rdf:type
//   - Blank nodes (_:label and [] syntax)
//   - String literals (quoted, triple-quoted, multiline)
//   - Typed literals (^^datatype)
//   - Language-tagged literals (@en, @de, etc.)
//   - Numeric and boolean literals
//
// Example:
//
//	parser := ttl.NewTurtleParser()
//	parser.Graph = "https://example.org/data"
//	triples, err := parser.Parse(reader)
//	for _, t := range triples {
//	    fmt.Println(t.Subject, t.Predicate, t.Object)
//	}
package ttl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soypete/ontology-go/types"
)

// TurtleParser parses W3C Turtle serialization into triples.
// It supports @prefix/@base declarations, prefixed names, full IRIs,
// predicate lists (;), object lists (,), the 'a' shorthand for rdf:type,
// blank nodes (_:label and [] syntax), string literals (quoted and triple-quoted),
// typed literals (^^), and language-tagged literals (@lang).
type TurtleParser struct {
	// Graph is the named graph to assign to all parsed triples.
	// If empty, the Triple.Graph field will be left blank.
	Graph string
}

// NewTurtleParser creates a new Turtle parser.
func NewTurtleParser() *TurtleParser {
	return &TurtleParser{}
}

// Parse reads Turtle from r and returns all triples.
func (p *TurtleParser) Parse(r io.Reader) ([]types.Triple, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("turtle: read error: %w", err)
	}
	return p.parse(string(data))
}

// ParseFile is a convenience wrapper for os.Open + Parse.
func (p *TurtleParser) ParseFile(path string) ([]types.Triple, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("turtle: open error: %w", err)
	}
	defer f.Close()
	return p.Parse(bufio.NewReader(f))
}

// turtleState holds the parser state.
type turtleState struct {
	input    string
	pos      int
	prefixes map[string]string // prefix -> IRI
	base     string
	graph    string
	bnode    int
}

// literalInfo holds metadata for RDF literal values.
type literalInfo struct {
	value     string
	isLiteral bool
	datatype  string
	lang      string
}

func (s *turtleState) newBlankNode() string {
	s.bnode++
	return fmt.Sprintf("_:tb%d", s.bnode)
}

func (p *TurtleParser) parse(input string) ([]types.Triple, error) {
	s := &turtleState{
		input:    input,
		prefixes: make(map[string]string),
		graph:    p.Graph,
	}

	var triples []types.Triple

	for {
		s.skipWS()
		if s.pos >= len(s.input) {
			break
		}

		// Check for directives
		if s.startsWith("@prefix") {
			if err := s.parsePrefix(); err != nil {
				return nil, err
			}
			continue
		}
		if s.startsWithCI("PREFIX") && !s.startsWithPName() {
			if err := s.parseSPARQLPrefix(); err != nil {
				return nil, err
			}
			continue
		}
		if s.startsWith("@base") {
			if err := s.parseBase(); err != nil {
				return nil, err
			}
			continue
		}
		if s.startsWithCI("BASE") && !s.startsWithPName() {
			if err := s.parseSPARQLBase(); err != nil {
				return nil, err
			}
			continue
		}

		// Parse a triple statement (subject predicateObjectList '.')
		parsed, err := s.parseTripleStatement()
		if err != nil {
			return nil, err
		}
		triples = append(triples, parsed...)
	}

	return triples, nil
}

// startsWith checks if the remaining input starts with the given string.
func (s *turtleState) startsWith(prefix string) bool {
	return strings.HasPrefix(s.input[s.pos:], prefix)
}

// startsWithCI checks case-insensitive prefix match followed by whitespace.
func (s *turtleState) startsWithCI(prefix string) bool {
	remaining := s.input[s.pos:]
	if len(remaining) < len(prefix) {
		return false
	}
	if !strings.EqualFold(remaining[:len(prefix)], prefix) {
		return false
	}
	// Must be followed by whitespace or end
	if len(remaining) > len(prefix) {
		r, _ := utf8.DecodeRuneInString(remaining[len(prefix):])
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// startsWithPName checks if current position looks like a prefixed name (e.g., PREFIX:something).
func (s *turtleState) startsWithPName() bool {
	remaining := s.input[s.pos:]
	for i, r := range remaining {
		if r == ':' {
			return i > 0
		}
		if unicode.IsSpace(r) {
			return false
		}
	}
	return false
}

func (s *turtleState) skipWS() {
	for s.pos < len(s.input) {
		r, size := utf8.DecodeRuneInString(s.input[s.pos:])
		if unicode.IsSpace(r) {
			s.pos += size
			continue
		}
		// Skip comments
		if r == '#' {
			s.pos += size
			for s.pos < len(s.input) {
				r2, size2 := utf8.DecodeRuneInString(s.input[s.pos:])
				s.pos += size2
				if r2 == '\n' || r2 == '\r' {
					break
				}
			}
			continue
		}
		break
	}
}

func (s *turtleState) parsePrefix() error {
	// @prefix prefix: <iri> .
	s.pos += len("@prefix")
	s.skipWS()

	prefix, err := s.readPrefixLabel()
	if err != nil {
		return err
	}

	s.skipWS()
	iri, err := s.readIRIRef()
	if err != nil {
		return fmt.Errorf("turtle: expected IRI in @prefix at pos %d: %w", s.pos, err)
	}

	s.skipWS()
	if err := s.expect('.'); err != nil {
		return fmt.Errorf("turtle: expected '.' after @prefix at pos %d: %w", s.pos, err)
	}

	s.prefixes[prefix] = s.resolveIRI(iri)
	return nil
}

func (s *turtleState) parseSPARQLPrefix() error {
	// PREFIX prefix: <iri>
	s.pos += len("PREFIX")
	s.skipWS()

	prefix, err := s.readPrefixLabel()
	if err != nil {
		return err
	}

	s.skipWS()
	iri, err := s.readIRIRef()
	if err != nil {
		return fmt.Errorf("turtle: expected IRI in PREFIX at pos %d: %w", s.pos, err)
	}

	s.prefixes[prefix] = s.resolveIRI(iri)
	return nil
}

func (s *turtleState) parseBase() error {
	// @base <iri> .
	s.pos += len("@base")
	s.skipWS()

	iri, err := s.readIRIRef()
	if err != nil {
		return fmt.Errorf("turtle: expected IRI in @base at pos %d: %w", s.pos, err)
	}

	s.skipWS()
	if err := s.expect('.'); err != nil {
		return fmt.Errorf("turtle: expected '.' after @base at pos %d: %w", s.pos, err)
	}

	s.base = iri
	return nil
}

func (s *turtleState) parseSPARQLBase() error {
	// BASE <iri>
	s.pos += len("BASE")
	s.skipWS()

	iri, err := s.readIRIRef()
	if err != nil {
		return fmt.Errorf("turtle: expected IRI in BASE at pos %d: %w", s.pos, err)
	}

	s.base = iri
	return nil
}

func (s *turtleState) readPrefixLabel() (string, error) {
	start := s.pos
	for s.pos < len(s.input) {
		r, size := utf8.DecodeRuneInString(s.input[s.pos:])
		if r == ':' {
			label := s.input[start:s.pos]
			s.pos += size // consume ':'
			return label, nil
		}
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("turtle: invalid prefix label at pos %d", start)
		}
		s.pos += size
	}
	return "", fmt.Errorf("turtle: unterminated prefix label at pos %d", start)
}

func (s *turtleState) readIRIRef() (string, error) {
	if s.pos >= len(s.input) || s.input[s.pos] != '<' {
		return "", fmt.Errorf("expected '<'")
	}
	s.pos++ // consume '<'
	start := s.pos
	for s.pos < len(s.input) {
		if s.input[s.pos] == '>' {
			iri := s.input[start:s.pos]
			s.pos++ // consume '>'
			return iri, nil
		}
		s.pos++
	}
	return "", fmt.Errorf("turtle: unterminated IRI at pos %d", start)
}

func (s *turtleState) resolveIRI(iri string) string {
	if s.base != "" && !strings.Contains(iri, "://") {
		return s.base + iri
	}
	return iri
}

func (s *turtleState) expect(ch byte) error {
	if s.pos >= len(s.input) || s.input[s.pos] != ch {
		return fmt.Errorf("expected '%c'", ch)
	}
	s.pos++
	return nil
}

func (s *turtleState) parseTripleStatement() ([]types.Triple, error) {
	var triples []types.Triple

	// Parse subject
	subject, subjectTriples, err := s.parseSubject()
	if err != nil {
		return nil, err
	}
	triples = append(triples, subjectTriples...)

	s.skipWS()

	// Parse predicate-object list
	poTriples, err := s.parsePredicateObjectList(subject)
	if err != nil {
		return nil, err
	}
	triples = append(triples, poTriples...)

	s.skipWS()
	if err := s.expect('.'); err != nil {
		return nil, fmt.Errorf("turtle: expected '.' at end of statement at pos %d: %w", s.pos, err)
	}

	return triples, nil
}

func (s *turtleState) parseSubject() (string, []types.Triple, error) {
	s.skipWS()
	if s.pos >= len(s.input) {
		return "", nil, fmt.Errorf("turtle: unexpected end of input at pos %d", s.pos)
	}

	ch := s.input[s.pos]

	// Full IRI
	if ch == '<' {
		iri, err := s.readIRIRef()
		if err != nil {
			return "", nil, err
		}
		return s.resolveIRI(iri), nil, nil
	}

	// Blank node [] syntax
	if ch == '[' {
		return s.parseBlankNodePropertyList()
	}

	// Blank node _:label
	if s.startsWith("_:") {
		return s.readBlankNodeLabel(), nil, nil
	}

	// Prefixed name or 'a'
	return s.readPrefixedNameOrKeyword()
}

func (s *turtleState) parsePredicate() (string, error) {
	s.skipWS()
	if s.pos >= len(s.input) {
		return "", fmt.Errorf("turtle: unexpected end of input at pos %d", s.pos)
	}

	// 'a' shorthand for rdf:type
	if s.pos < len(s.input) {
		remaining := s.input[s.pos:]
		if len(remaining) >= 1 && remaining[0] == 'a' {
			if len(remaining) == 1 || unicode.IsSpace(rune(remaining[1])) || remaining[1] == '<' || remaining[1] == '_' || remaining[1] == '[' || remaining[1] == '"' || remaining[1] == '(' {
				s.pos++
				return types.RDFType, nil
			}
		}
	}

	ch := s.input[s.pos]

	// Full IRI
	if ch == '<' {
		iri, err := s.readIRIRef()
		if err != nil {
			return "", err
		}
		return s.resolveIRI(iri), nil
	}

	// Prefixed name
	name, _, err := s.readPrefixedNameOrKeyword()
	return name, err
}

func (s *turtleState) parseObject() (string, literalInfo, []types.Triple, error) {
	s.skipWS()
	if s.pos >= len(s.input) {
		return "", literalInfo{}, nil, fmt.Errorf("turtle: unexpected end of input at pos %d", s.pos)
	}

	ch := s.input[s.pos]

	// Full IRI
	if ch == '<' {
		iri, err := s.readIRIRef()
		if err != nil {
			return "", literalInfo{}, nil, err
		}
		return s.resolveIRI(iri), literalInfo{}, nil, nil
	}

	// Blank node [] syntax
	if ch == '[' {
		bnode, triples, err := s.parseBlankNodePropertyList()
		return bnode, literalInfo{}, triples, err
	}

	// RDF collection () syntax
	if ch == '(' {
		bnode, triples, err := s.parseCollection()
		return bnode, literalInfo{}, triples, err
	}

	// Blank node _:label
	if s.startsWith("_:") {
		return s.readBlankNodeLabel(), literalInfo{}, nil, nil
	}

	// String literal
	if ch == '"' || ch == '\'' {
		lit, triples, err := s.readLiteral()
		return lit.value, lit, triples, err
	}

	// Boolean or numeric literals
	if ch == '+' || ch == '-' || (ch >= '0' && ch <= '9') {
		value := s.readNumericLiteral()
		lit := literalInfo{
			value:     value,
			isLiteral: true,
			datatype:  types.XSDDecimal,
		}
		if strings.Contains(value, ".") {
			lit.datatype = types.XSDDouble
		}
		return value, lit, nil, nil
	}
	if s.startsWith("true") || s.startsWith("false") {
		value := s.readBooleanLiteral()
		lit := literalInfo{
			value:     value,
			isLiteral: true,
			datatype:  types.XSDBoolean,
		}
		return value, lit, nil, nil
	}

	// Prefixed name
	name, triples, err := s.readPrefixedNameOrKeyword()
	return name, literalInfo{}, triples, err
}

func (s *turtleState) parsePredicateObjectList(subject string) ([]types.Triple, error) {
	var triples []types.Triple

	for {
		s.skipWS()
		if s.pos >= len(s.input) {
			break
		}

		// Check for end of statement
		if s.input[s.pos] == '.' || s.input[s.pos] == ']' {
			break
		}

		predicate, err := s.parsePredicate()
		if err != nil {
			return nil, err
		}

		// Parse object list
		objTriples, err := s.parseObjectList(subject, predicate)
		if err != nil {
			return nil, err
		}
		triples = append(triples, objTriples...)

		s.skipWS()
		if s.pos >= len(s.input) {
			break
		}

		// ';' separates predicate-object pairs
		if s.input[s.pos] == ';' {
			s.pos++
			s.skipWS()
			// Handle trailing semicolons before '.' or ']'
			if s.pos < len(s.input) && (s.input[s.pos] == '.' || s.input[s.pos] == ']') {
				break
			}
			continue
		}

		break
	}

	return triples, nil
}

func (s *turtleState) parseObjectList(subject, predicate string) ([]types.Triple, error) {
	var triples []types.Triple

	for {
		s.skipWS()

		object, lit, objTriples, err := s.parseObject()
		if err != nil {
			return nil, err
		}
		triples = append(triples, objTriples...)
		triples = append(triples, types.Triple{
			Subject:   subject,
			Predicate: predicate,
			Object:    object,
			Graph:     s.graph,
			IsLiteral: lit.isLiteral,
			Datatype:  lit.datatype,
			Lang:      lit.lang,
		})

		s.skipWS()
		if s.pos >= len(s.input) || s.input[s.pos] != ',' {
			break
		}
		s.pos++
	}

	return triples, nil
}

func (s *turtleState) parseCollection() (string, []types.Triple, error) {
	s.pos++
	s.skipWS()

	var triples []types.Triple
	var items []string
	var litInfos []literalInfo

	for {
		s.skipWS()
		if s.pos >= len(s.input) {
			return "", nil, fmt.Errorf("turtle: unterminated collection at pos %d", s.pos)
		}

		if s.input[s.pos] == ')' {
			s.pos++
			break
		}

		item, lit, itemTriples, err := s.parseObject()
		if err != nil {
			return "", nil, err
		}
		items = append(items, item)
		litInfos = append(litInfos, lit)
		triples = append(triples, itemTriples...)
	}

	if len(items) == 0 {
		return types.RDFNil, nil, nil
	}

	var head string
	for i := len(items) - 1; i >= 0; i-- {
		bnode := s.newBlankNode()
		triple := types.Triple{
			Subject:   bnode,
			Predicate: types.RDFFirst,
			Object:    items[i],
			Graph:     s.graph,
		}
		if i < len(litInfos) {
			triple.IsLiteral = litInfos[i].isLiteral
			triple.Datatype = litInfos[i].datatype
			triple.Lang = litInfos[i].lang
		}
		triples = append(triples, triple)

		if i == len(items)-1 {
			triples = append(triples, types.Triple{
				Subject:   bnode,
				Predicate: types.RDFRest,
				Object:    types.RDFNil,
				Graph:     s.graph,
			})
		} else {
			triples = append(triples, types.Triple{
				Subject:   bnode,
				Predicate: types.RDFRest,
				Object:    head,
				Graph:     s.graph,
			})
		}

		head = bnode
	}

	return head, triples, nil
}

func (s *turtleState) parseBlankNodePropertyList() (string, []types.Triple, error) {
	s.pos++ // consume '['
	s.skipWS()

	bnode := s.newBlankNode()

	// Empty blank node []
	if s.pos < len(s.input) && s.input[s.pos] == ']' {
		s.pos++
		return bnode, nil, nil
	}

	triples, err := s.parsePredicateObjectList(bnode)
	if err != nil {
		return "", nil, err
	}

	s.skipWS()
	if err := s.expect(']'); err != nil {
		return "", nil, fmt.Errorf("turtle: expected ']' at pos %d: %w", s.pos, err)
	}

	return bnode, triples, nil
}

func (s *turtleState) readBlankNodeLabel() string {
	s.pos += 2 // consume '_:'
	start := s.pos
	for s.pos < len(s.input) {
		r, size := utf8.DecodeRuneInString(s.input[s.pos:])
		if isNameChar(r) {
			s.pos += size
		} else {
			break
		}
	}
	return "_:" + s.input[start:s.pos]
}

func (s *turtleState) readPrefixedNameOrKeyword() (string, []types.Triple, error) {
	start := s.pos

	// Read the prefix part (before ':')
	for s.pos < len(s.input) {
		r, size := utf8.DecodeRuneInString(s.input[s.pos:])
		if r == ':' {
			prefix := s.input[start:s.pos]
			s.pos += size // consume ':'

			// Read local part
			localStart := s.pos
			for s.pos < len(s.input) {
				r2, size2 := utf8.DecodeRuneInString(s.input[s.pos:])
				if isNameChar(r2) || r2 == '.' || r2 == '-' {
					// Don't include trailing dots
					if r2 == '.' {
						// Look ahead - only include if followed by namechar
						nextPos := s.pos + size2
						if nextPos < len(s.input) {
							nr, _ := utf8.DecodeRuneInString(s.input[nextPos:])
							if !isNameChar(nr) && nr != '.' && nr != '-' {
								break
							}
						} else {
							break
						}
					}
					s.pos += size2
				} else {
					break
				}
			}
			local := s.input[localStart:s.pos]

			ns, ok := s.prefixes[prefix]
			if !ok {
				return "", nil, fmt.Errorf("turtle: unknown prefix '%s' at pos %d", prefix, start)
			}
			return ns + local, nil, nil
		}
		if !isNameChar(r) && r != '.' && r != '-' {
			break
		}
		s.pos += size
	}

	// Not a prefixed name - might be a keyword that shouldn't be here
	word := s.input[start:s.pos]
	return "", nil, fmt.Errorf("turtle: unexpected token '%s' at pos %d", word, start)
}

func (s *turtleState) readLiteral() (literalInfo, []types.Triple, error) {
	quote := s.input[s.pos]
	var value string
	var err error

	if s.pos+2 < len(s.input) && s.input[s.pos:s.pos+3] == string([]byte{quote, quote, quote}) {
		value, err = s.readTripleQuotedString(quote)
	} else {
		value, err = s.readQuotedString(quote)
	}
	if err != nil {
		return literalInfo{}, nil, err
	}

	lit := literalInfo{
		value:     value,
		isLiteral: true,
		datatype:  types.XSDString,
	}

	if s.pos < len(s.input) && s.input[s.pos] == '^' && s.pos+1 < len(s.input) && s.input[s.pos+1] == '^' {
		s.pos += 2
		if s.pos < len(s.input) && s.input[s.pos] == '<' {
			datatype, err := s.readIRIRef()
			if err != nil {
				return literalInfo{}, nil, err
			}
			lit.datatype = s.resolveIRI(datatype)
		} else {
			datatype, _, err := s.readPrefixedNameOrKeyword()
			if err != nil {
				return literalInfo{}, nil, err
			}
			lit.datatype = datatype
		}
		return lit, nil, nil
	}

	if s.pos < len(s.input) && s.input[s.pos] == '@' {
		s.pos++
		var lang strings.Builder
		for s.pos < len(s.input) {
			r, size := utf8.DecodeRuneInString(s.input[s.pos:])
			if r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				lang.WriteRune(r)
				s.pos += size
			} else {
				break
			}
		}
		lit.lang = lang.String()
		lit.datatype = ""
		return lit, nil, nil
	}

	return lit, nil, nil
}

func (s *turtleState) readQuotedString(quote byte) (string, error) {
	s.pos++ // consume opening quote
	var b strings.Builder
	for s.pos < len(s.input) {
		ch := s.input[s.pos]
		if ch == '\\' {
			s.pos++
			if s.pos >= len(s.input) {
				return "", fmt.Errorf("turtle: unterminated escape at pos %d", s.pos)
			}
			escaped := s.input[s.pos]
			switch escaped {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '\'':
				b.WriteByte('\'')
			default:
				b.WriteByte('\\')
				b.WriteByte(escaped)
			}
			s.pos++
			continue
		}
		if ch == quote {
			s.pos++ // consume closing quote
			return b.String(), nil
		}
		b.WriteByte(ch)
		s.pos++
	}
	return "", fmt.Errorf("turtle: unterminated string at pos %d", s.pos)
}

func (s *turtleState) readTripleQuotedString(quote byte) (string, error) {
	s.pos += 3 // consume opening triple quote
	end := string([]byte{quote, quote, quote})
	var b strings.Builder
	for s.pos < len(s.input) {
		if s.pos+2 < len(s.input) && s.input[s.pos:s.pos+3] == end {
			s.pos += 3 // consume closing triple quote
			return b.String(), nil
		}
		ch := s.input[s.pos]
		if ch == '\\' {
			s.pos++
			if s.pos >= len(s.input) {
				return "", fmt.Errorf("turtle: unterminated escape in triple-quoted string at pos %d", s.pos)
			}
			escaped := s.input[s.pos]
			switch escaped {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '\'':
				b.WriteByte('\'')
			default:
				b.WriteByte('\\')
				b.WriteByte(escaped)
			}
			s.pos++
			continue
		}
		b.WriteByte(ch)
		s.pos++
	}
	return "", fmt.Errorf("turtle: unterminated triple-quoted string at pos %d", s.pos)
}

func (s *turtleState) readNumericLiteral() string {
	start := s.pos
	if s.pos < len(s.input) && (s.input[s.pos] == '+' || s.input[s.pos] == '-') {
		s.pos++
	}
	for s.pos < len(s.input) && s.input[s.pos] >= '0' && s.input[s.pos] <= '9' {
		s.pos++
	}
	if s.pos < len(s.input) && s.input[s.pos] == '.' {
		s.pos++
		for s.pos < len(s.input) && s.input[s.pos] >= '0' && s.input[s.pos] <= '9' {
			s.pos++
		}
	}
	// Exponent
	if s.pos < len(s.input) && (s.input[s.pos] == 'e' || s.input[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.input) && (s.input[s.pos] == '+' || s.input[s.pos] == '-') {
			s.pos++
		}
		for s.pos < len(s.input) && s.input[s.pos] >= '0' && s.input[s.pos] <= '9' {
			s.pos++
		}
	}
	return s.input[start:s.pos]
}

func (s *turtleState) readBooleanLiteral() string {
	if s.startsWith("true") {
		s.pos += 4
		return "true"
	}
	s.pos += 5
	return "false"
}

// isNameChar returns true if r is a valid character in a Turtle local name or prefix.
func isNameChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '%'
}
