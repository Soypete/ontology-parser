package ttlast

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Parser struct {
	doc      *Document
	input    string
	pos      int
	prefixes map[string]string
	base     string
	bnode    int
}

func NewParser() *Parser {
	return &Parser{
		doc:      &Document{},
		prefixes: make(map[string]string),
	}
}

func (p *Parser) Parse(r io.Reader) (*Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("ttlast: read error: %w", err)
	}
	return p.parse(string(data))
}

func (p *Parser) ParseFile(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ttlast: open error: %w", err)
	}
	defer func() { _ = f.Close() }()
	return p.Parse(f)
}

func (p *Parser) parse(input string) (*Document, error) {
	p.input = input
	p.pos = 0
	p.doc.Input = input

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}

		if p.startsWith("@prefix") {
			if err := p.parsePrefix(); err != nil {
				return nil, err
			}
			continue
		}
		if p.startsWithCI("PREFIX") && !p.startsWithPName() {
			if err := p.parseSPARQLPrefix(); err != nil {
				return nil, err
			}
			continue
		}
		if p.startsWith("@base") {
			if err := p.parseBase(); err != nil {
				return nil, err
			}
			continue
		}
		if p.startsWithCI("BASE") && !p.startsWithPName() {
			if err := p.parseSPARQLBase(); err != nil {
				return nil, err
			}
			continue
		}

		if p.input[p.pos] == '#' {
			p.parseComment()
			continue
		}

		parsed, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			p.doc.Statements = append(p.doc.Statements, parsed...)
		}
	}

	return p.doc, nil
}

func (p *Parser) startsWith(prefix string) bool {
	return strings.HasPrefix(p.input[p.pos:], prefix)
}

func (p *Parser) startsWithCI(prefix string) bool {
	remaining := p.input[p.pos:]
	if len(remaining) < len(prefix) {
		return false
	}
	if !strings.EqualFold(remaining[:len(prefix)], prefix) {
		return false
	}
	if len(remaining) > len(prefix) {
		r, _ := utf8.DecodeRuneInString(remaining[len(prefix):])
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func (p *Parser) startsWithPName() bool {
	remaining := p.input[p.pos:]
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

func (p *Parser) skipWS() {
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if unicode.IsSpace(r) {
			p.pos += size
			continue
		}
		if r == '#' {
			p.parseComment()
			continue
		}
		break
	}
}

func (p *Parser) parseComment() {
	start := p.pos
	p.pos++
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		p.pos += size
		if r == '\n' || r == '\r' {
			break
		}
	}
	p.doc.Comments = append(p.doc.Comments, Comment{
		Base: Base{pos: start, end: p.pos},
		Text: p.input[start:p.pos],
	})
}

func (p *Parser) parsePrefix() error {
	start := p.pos
	p.pos += len("@prefix")
	p.skipWS()

	prefix, err := p.readPrefixLabel()
	if err != nil {
		return err
	}

	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return fmt.Errorf("ttlast: expected IRI in @prefix at pos %d: %w", p.pos, err)
	}

	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '.' {
		return fmt.Errorf("ttlast: expected '.' after @prefix at pos %d", p.pos)
	}
	p.pos++

	p.prefixes[prefix] = iri.Value

	p.doc.Prefixes = append(p.doc.Prefixes, PrefixDecl{
		Base:   Base{pos: start, end: p.pos},
		Prefix: prefix,
		IRI:    *iri,
	})
	return nil
}

func (p *Parser) parseSPARQLPrefix() error {
	start := p.pos
	p.pos += len("PREFIX")
	p.skipWS()

	prefix, err := p.readPrefixLabel()
	if err != nil {
		return err
	}

	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return fmt.Errorf("ttlast: expected IRI in PREFIX at pos %d: %w", p.pos, err)
	}

	p.prefixes[prefix] = iri.Value

	p.doc.Prefixes = append(p.doc.Prefixes, PrefixDecl{
		Base:   Base{pos: start, end: p.pos},
		Prefix: prefix,
		IRI:    *iri,
	})
	return nil
}

func (p *Parser) parseBase() error {
	p.pos += len("@base")
	p.skipWS()

	iri, err := p.readIRI()
	if err != nil {
		return fmt.Errorf("ttlast: expected IRI in @base at pos %d: %w", p.pos, err)
	}

	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '.' {
		return fmt.Errorf("ttlast: expected '.' after @base at pos %d", p.pos)
	}
	p.pos++

	p.base = iri.Value
	return nil
}

func (p *Parser) parseSPARQLBase() error {
	p.pos += len("BASE")
	p.skipWS()

	iri, err := p.readIRI()
	if err != nil {
		return fmt.Errorf("ttlast: expected IRI in BASE at pos %d: %w", p.pos, err)
	}

	p.base = iri.Value
	return nil
}

func (p *Parser) readPrefixLabel() (string, error) {
	start := p.pos
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r == ':' {
			label := p.input[start:p.pos]
			p.pos += size
			return label, nil
		}
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("ttlast: invalid prefix label at pos %d", start)
		}
		p.pos += size
	}
	return "", fmt.Errorf("ttlast: unterminated prefix label at pos %d", start)
}

func (p *Parser) readIRI() (*IRI, error) {
	if p.pos >= len(p.input) || p.input[p.pos] != '<' {
		return nil, fmt.Errorf("ttlast: expected '<'")
	}
	start := p.pos
	p.pos++
	iriStart := p.pos
	for p.pos < len(p.input) {
		if p.input[p.pos] == '>' {
			iri := p.input[iriStart:p.pos]
			p.pos++
			return &IRI{Base: Base{pos: start, end: p.pos}, Value: iri}, nil
		}
		p.pos++
	}
	return nil, fmt.Errorf("ttlast: unterminated IRI at pos %d", start)
}

func (p *Parser) parseStatement() ([]Statement, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, nil
	}

	if p.input[p.pos] == '.' {
		p.pos++
		return nil, nil
	}

	start := p.pos

	subject, err := p.parseSubject()
	if err != nil {
		return nil, err
	}

	var statements []Statement

	for {
		p.skipWS()

		predicate, err := p.parsePredicate()
		if err != nil {
			return nil, err
		}

		p.skipWS()

		for {
			object, err := p.parseObject()
			if err != nil {
				return nil, err
			}

			stmtStart := start
			stmt := Statement{
				Base: Base{pos: stmtStart, end: p.pos},
				Triple: Triple{
					Base:      Base{pos: stmtStart, end: p.pos},
					Subject:   subject,
					Predicate: predicate,
					Object:    object,
				},
			}
			statements = append(statements, stmt)

			p.skipWS()
			if p.pos >= len(p.input) || p.input[p.pos] != ',' {
				break
			}
			p.pos++
		}

		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ';' {
			break
		}
		p.pos++
	}

	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '.' {
		if len(statements) > 0 {
			return statements, nil
		}
		return nil, fmt.Errorf("ttlast: expected '.' or ';' at pos %d", p.pos)
	}
	p.pos++

	if len(statements) == 0 {
		return nil, nil
	}

	for i := range statements {
		statements[i].Base.end = p.pos
		statements[i].Triple.Base.end = p.pos
	}
	return statements, nil
}

func (p *Parser) parseSubject() (Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("ttlast: unexpected end of input at pos %d", p.pos)
	}

	ch := p.input[p.pos]

	if ch == '<' {
		return p.readIRI()
	}

	if p.startsWith("_:") {
		return p.readBlankNode()
	}

	if ch == '[' {
		return p.parseBlankNodePropertyList()
	}

	if ch == '(' {
		return p.parseCollection()
	}

	return p.parsePrefixedNameOrKeyword()
}

func (p *Parser) parsePredicate() (Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("ttlast: unexpected end of input at pos %d", p.pos)
	}

	if p.pos < len(p.input) && p.input[p.pos] == 'a' {
		remaining := p.input[p.pos:]
		if len(remaining) == 1 || unicode.IsSpace(rune(remaining[1])) {
			p.pos++
			return &PrefixedName{
				Base:   Base{pos: p.pos - 1, end: p.pos},
				Prefix: "rdf",
				Local:  "type",
			}, nil
		}
	}

	ch := p.input[p.pos]

	if ch == '<' {
		return p.readIRI()
	}

	return p.parsePrefixedNameOrKeyword()
}

func (p *Parser) parseObject() (Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("ttlast: unexpected end of input at pos %d", p.pos)
	}

	ch := p.input[p.pos]

	if ch == '<' {
		return p.readIRI()
	}

	if p.startsWith("_:") {
		return p.readBlankNode()
	}

	if ch == '[' {
		return p.parseBlankNodePropertyList()
	}

	if ch == '(' {
		return p.parseCollection()
	}

	if ch == '"' || ch == '\'' {
		return p.readLiteral()
	}

	if ch == '+' || ch == '-' || (ch >= '0' && ch <= '9') {
		return p.readNumericLiteral()
	}

	if p.startsWith("true") || p.startsWith("false") {
		return p.readBooleanLiteral()
	}

	return p.parsePrefixedNameOrKeyword()
}

func (p *Parser) parsePrefixedNameOrKeyword() (Term, error) {
	start := p.pos

	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r == ':' {
			prefix := p.input[start:p.pos]
			p.pos += size

			localStart := p.pos
			for p.pos < len(p.input) {
				r2, size2 := utf8.DecodeRuneInString(p.input[p.pos:])
				if isNameChar(r2) || r2 == '.' || r2 == '-' {
					p.pos += size2
				} else {
					break
				}
			}
			local := p.input[localStart:p.pos]

			_, ok := p.prefixes[prefix]
			if !ok {
				return nil, fmt.Errorf("ttlast: unknown prefix '%s' at pos %d", prefix, start)
			}
			return &PrefixedName{
				Base:   Base{pos: start, end: p.pos},
				Prefix: prefix,
				Local:  local,
			}, nil
		}
		if !isNameChar(r) && r != '.' && r != '-' {
			break
		}
		p.pos += size
	}

	word := p.input[start:p.pos]
	return nil, fmt.Errorf("ttlast: unexpected token '%s' at pos %d", word, start)
}

func (p *Parser) readBlankNode() (*BlankNode, error) {
	start := p.pos
	p.pos += 2
	labelStart := p.pos
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if isNameChar(r) {
			p.pos += size
		} else {
			break
		}
	}
	return &BlankNode{
		Base:  Base{pos: start, end: p.pos},
		Label: p.input[labelStart:p.pos],
	}, nil
}

func (p *Parser) parseBlankNodePropertyList() (*BlankNode, error) {
	start := p.pos
	p.pos++
	p.skipWS()

	bnode := p.newBlankNode()
	bn := &BlankNode{Base: Base{pos: start, end: start + 2}, Label: bnode}

	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		bn.Base.end = p.pos
		return bn, nil
	}

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}
		if p.input[p.pos] == ']' {
			p.pos++
			break
		}

		_, err := p.parsePredicate()
		if err != nil {
			return nil, err
		}

		p.skipWS()

		_, err = p.parseObject()
		if err != nil {
			return nil, err
		}

		p.skipWS()

		if p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ']' {
				break
			}
			continue
		}
		break
	}

	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
	}
	bn.Base.end = p.pos

	return bn, nil
}

func (p *Parser) parseCollection() (*Collection, error) {
	start := p.pos
	p.pos++
	p.skipWS()

	var elements []Term

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("ttlast: unterminated collection at pos %d", p.pos)
		}

		if p.input[p.pos] == ')' {
			p.pos++
			break
		}

		elem, err := p.parseObject()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}

	return &Collection{
		Base:     Base{pos: start, end: p.pos},
		Elements: elements,
	}, nil
}

func (p *Parser) readLiteral() (*Literal, error) {
	start := p.pos
	quote := p.input[p.pos]

	var value string
	var err error

	if p.pos+2 < len(p.input) && p.input[p.pos:p.pos+3] == string([]byte{quote, quote, quote}) {
		value, err = p.readTripleQuotedString(quote)
	} else {
		value, err = p.readQuotedString(quote)
	}
	if err != nil {
		return nil, err
	}

	lit := &Literal{
		Base:     Base{pos: start, end: p.pos},
		Value:    value,
		Datatype: "http://www.w3.org/2001/XMLSchema#string",
	}

	if p.pos < len(p.input) && p.input[p.pos] == '^' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '^' {
		p.pos += 2
		if p.pos < len(p.input) && p.input[p.pos] == '<' {
			dt, err := p.readIRI()
			if err != nil {
				return nil, err
			}
			lit.Datatype = dt.Value
		} else {
			dt, err := p.parsePrefixedNameOrKeyword()
			if err != nil {
				return nil, err
			}
			if pn, ok := dt.(*PrefixedName); ok {
				lit.Datatype = pn.Prefix + pn.Local
			}
		}
	}

	if p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++
		var lang strings.Builder
		for p.pos < len(p.input) {
			r, size := utf8.DecodeRuneInString(p.input[p.pos:])
			if r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				lang.WriteRune(r)
				p.pos += size
			} else {
				break
			}
		}
		langStr := lang.String()
		if strings.HasSuffix(langStr, "--ltr") || strings.HasSuffix(langStr, "--rtl") {
			parts := strings.Split(langStr, "--")
			lit.Language = parts[0]
			lit.Direction = parts[1]
			lit.Datatype = "http://www.w3.org/1999/02/22-rdf-syntax-ns#dirLangString"
		} else {
			lit.Language = langStr
			lit.Datatype = "http://www.w3.org/1999/02/22-rdf-syntax-ns#langString"
		}
	}

	return lit, nil
}

func (p *Parser) readQuotedString(quote byte) (string, error) {
	p.pos++
	var b strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("ttlast: unterminated escape at pos %d", p.pos)
			}
			escaped := p.input[p.pos]
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
			p.pos++
			continue
		}
		if ch == quote {
			p.pos++
			return b.String(), nil
		}
		b.WriteByte(ch)
		p.pos++
	}
	return "", fmt.Errorf("ttlast: unterminated string at pos %d", p.pos)
}

func (p *Parser) readTripleQuotedString(quote byte) (string, error) {
	p.pos += 3
	end := string([]byte{quote, quote, quote})
	var b strings.Builder
	for p.pos < len(p.input) {
		if p.pos+2 < len(p.input) && p.input[p.pos:p.pos+3] == end {
			p.pos += 3
			return b.String(), nil
		}
		ch := p.input[p.pos]
		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("ttlast: unterminated escape in triple-quoted string at pos %d", p.pos)
			}
			escaped := p.input[p.pos]
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
			p.pos++
			continue
		}
		b.WriteByte(ch)
		p.pos++
	}
	return "", fmt.Errorf("ttlast: unterminated triple-quoted string at pos %d", p.pos)
}

func (p *Parser) readNumericLiteral() (*Literal, error) {
	start := p.pos
	if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		p.pos++
	}
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
	}
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}

	lexical := p.input[start:p.pos]
	var datatype string
	if strings.ContainsAny(lexical, ".eE") {
		if strings.Contains(lexical, "e") || strings.Contains(lexical, "E") {
			datatype = "http://www.w3.org/2001/XMLSchema#double"
		} else {
			datatype = "http://www.w3.org/2001/XMLSchema#decimal"
		}
	} else {
		datatype = "http://www.w3.org/2001/XMLSchema#integer"
	}

	return &Literal{
		Base:     Base{pos: start, end: p.pos},
		Value:    lexical,
		Datatype: datatype,
	}, nil
}

func (p *Parser) readBooleanLiteral() (*Literal, error) {
	start := p.pos
	if p.startsWith("true") {
		p.pos += 4
		return &Literal{
			Base:     Base{pos: start, end: p.pos},
			Value:    "true",
			Datatype: "http://www.w3.org/2001/XMLSchema#boolean",
		}, nil
	}
	p.pos += 5
	return &Literal{
		Base:     Base{pos: start, end: p.pos},
		Value:    "false",
		Datatype: "http://www.w3.org/2001/XMLSchema#boolean",
	}, nil
}

func (p *Parser) newBlankNode() string {
	p.bnode++
	return fmt.Sprintf("b%d", p.bnode)
}

func isNameChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '%'
}
