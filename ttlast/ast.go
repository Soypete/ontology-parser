// Package ttlast provides an AST representation of Turtle RDF files with source position tracking.
//
// This package parses Turtle (.ttl) files into an abstract syntax tree where each node
// tracks its byte offset position in the source file, enabling provenance attribution
// for inferred facts.
//
// The AST includes support for:
//   - @prefix declarations
//   - @base declarations
//   - Subject-predicate-object triples
//   - Blank nodes (_:label and [] syntax)
//   - Collections (a b c)
//   - Literals (string, numeric, boolean, typed, language-tagged)
//   - Comments (preserved for provenance)
//
// Example:
//
//	parser := ttlast.NewParser()
//	doc, err := parser.ParseFile("example.ttl")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Walk the AST
//	ttlast.Walk(ttlast.DefaultVisitor{}, doc)
//
//	// Get position info for provenance
//	for _, stmt := range doc.Statements {
//	    fmt.Printf("Triple at bytes %d-%d\n", stmt.Pos(), stmt.End())
//	}
package ttlast

import "fmt"

type Node interface {
	Pos() int
	End() int
}

type Base struct {
	pos int
	end int
}

func (b *Base) Pos() int     { return b.pos }
func (b *Base) End() int     { return b.end }
func (b *Base) SetPos(p int) { b.pos = p }
func (b *Base) SetEnd(e int) { b.end = e }

type Document struct {
	Base
	Input      string
	Prefixes   []PrefixDecl
	Statements []Statement
	Comments   []Comment
}

type PrefixDecl struct {
	Base
	Prefix string
	IRI    IRI
}

type Statement struct {
	Base
	Triple Triple
}

type Triple struct {
	Base
	Subject   Term
	Predicate Term
	Object    Term
}

type Comment struct {
	Base
	Text string
}

type IRI struct {
	Base
	Value string
}

type PrefixedName struct {
	Base
	Prefix string
	Local  string
}

type BlankNode struct {
	Base
	Label string
}

type Literal struct {
	Base
	Value     string
	Datatype  string
	Language  string
	Direction string
}

type Collection struct {
	Base
	Elements []Term
}

type List struct {
	Base
	Elements []Term
}

type Term interface {
	Node
	isTerm()
}

func (iri *IRI) isTerm()         {}
func (pn *PrefixedName) isTerm() {}
func (bn *BlankNode) isTerm()    {}
func (lit *Literal) isTerm()     {}
func (col *Collection) isTerm()  {}

func (d *Document) String() string {
	var s string
	for _, p := range d.Prefixes {
		s += fmt.Sprintf("@prefix %s: <%s> .\n", p.Prefix, p.IRI.Value)
	}
	for _, stmt := range d.Statements {
		s += fmt.Sprintf("%s %s %s .\n", stmt.Triple.Subject, stmt.Triple.Predicate, stmt.Triple.Object)
	}
	return s
}

func (iri *IRI) String() string         { return "<" + iri.Value + ">" }
func (pn *PrefixedName) String() string { return pn.Prefix + ":" + pn.Local }
func (bn *BlankNode) String() string    { return "_:" + bn.Label }
func (lit *Literal) String() string {
	if lit.Language != "" {
		return fmt.Sprintf("%q@%s", lit.Value, lit.Language)
	}
	if lit.Datatype != "" {
		return fmt.Sprintf("%q^^%s", lit.Value, lit.Datatype)
	}
	return fmt.Sprintf("%q", lit.Value)
}
func (col *Collection) String() string {
	s := "("
	for i, e := range col.Elements {
		if i > 0 {
			s += " "
		}
		s += termString(e)
	}
	s += ")"
	return s
}

func termString(t Term) string {
	switch v := t.(type) {
	case *IRI:
		return v.String()
	case *PrefixedName:
		return v.String()
	case *BlankNode:
		return v.String()
	case *Literal:
		return v.String()
	case *Collection:
		return v.String()
	default:
		return ""
	}
}
