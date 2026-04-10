// Package rdf provides RDF/OWL ontology parsing functionality.
//
// This package supports parsing RDF/XML, Turtle (TTL), and OWL ontology files
// into RDF triples. It provides a Parser interface for extensible format support,
// with an XMLParser implementation for RDF/XML documents.
//
// Triples are represented using the types.Triple struct and can be stored in
// a store.Store for querying. This package is typically used alongside the ttl
// and sparql packages for complete ontology workflow support.
//
// Example:
//
//	parser := rdf.NewXMLParser("https://example.org/graph")
//	triples, err := parser.Parse(file)
//	// Use triples with store.Store
package rdf

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/soypete/ontology-go/types"
)

// Parser defines the interface for parsing RDF formats into triples.
type Parser interface {
	Parse(r io.Reader) ([]types.Triple, error)
}

type XMLParser struct {
	Graph string
}

func NewXMLParser(graph string) *XMLParser {
	return &XMLParser{Graph: graph}
}

func (p *XMLParser) Parse(r io.Reader) ([]types.Triple, error) {
	decoder := xml.NewDecoder(r)
	var triples []types.Triple
	var err error

	for {
		tok, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("xml parse error: %w", e)
		}

		if se, ok := tok.(xml.StartElement); ok {
			if localName(se) == "RDF" && isRDFNamespace(se.Name.Space) {
				triples, err = p.parseRDFRoot(decoder)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	}

	return triples, nil
}

func (p *XMLParser) ParseString(s string) ([]types.Triple, error) {
	return p.Parse(strings.NewReader(s))
}

const (
	rdfNS   = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	xmlNS   = "http://www.w3.org/XML/1998/namespace"
	rdfType = rdfNS + "type"
)

func isRDFNamespace(ns string) bool {
	return ns == rdfNS
}

func localName(se xml.StartElement) string {
	return se.Name.Local
}

func (p *XMLParser) parseRDFRoot(decoder *xml.Decoder) ([]types.Triple, error) {
	var triples []types.Triple

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return triples, nil
		}
		if err != nil {
			return nil, fmt.Errorf("xml parse error: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			parsed, err := p.parseDescription(decoder, t, 0)
			if err != nil {
				return nil, err
			}
			triples = append(triples, parsed...)
		case xml.EndElement:
			return triples, nil
		}
	}
}

var blankNodeCounter int

func newBlankNode() string {
	blankNodeCounter++
	return fmt.Sprintf("_:b%d", blankNodeCounter)
}

func (p *XMLParser) parseDescription(decoder *xml.Decoder, el xml.StartElement, depth int) ([]types.Triple, error) {
	if depth > 50 {
		return nil, fmt.Errorf("rdf/xml nesting too deep (>50 levels)")
	}

	var triples []types.Triple

	subject := p.getSubject(el)

	if !(el.Name.Space == rdfNS && el.Name.Local == "Description") {
		typeURI := el.Name.Space + el.Name.Local
		triples = append(triples, p.triple(subject, rdfType, typeURI))
	}

	for _, attr := range el.Attr {
		if attr.Name.Space == rdfNS || attr.Name.Space == xmlNS {
			continue
		}
		if attr.Name.Space == "" && (attr.Name.Local == "about" || attr.Name.Local == "resource" ||
			attr.Name.Local == "nodeID" || attr.Name.Local == "ID" || attr.Name.Local == "lang") {
			continue
		}
		predURI := attr.Name.Space + attr.Name.Local
		if predURI != attr.Name.Local {
			triples = append(triples, p.triple(subject, predURI, attr.Value))
		}
	}

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return triples, nil
		}
		if err != nil {
			return nil, fmt.Errorf("xml parse error: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			propURI := t.Name.Space + t.Name.Local

			if resourceURI := getAttr(t, rdfNS, "resource"); resourceURI != "" {
				triples = append(triples, p.triple(subject, propURI, resourceURI))
				if err := skipElement(decoder); err != nil {
					return nil, err
				}
				continue
			}

			datatype := getAttr(t, rdfNS, "datatype")
			lang := getAttr(t, xmlNS, "lang")
			if lang == "" {
				lang = getAttrLocal(t, "lang")
			}

			innerTriples, objectValue, err := p.parsePropertyContent(decoder, t, subject, propURI, depth)
			if err != nil {
				return nil, err
			}

			if innerTriples != nil {
				triples = append(triples, innerTriples...)
			} else if objectValue != "" {
				if lang != "" {
					var dir string
					if strings.HasSuffix(lang, "--rtl") || strings.HasSuffix(lang, "--ltr") {
						parts := strings.Split(lang, "--")
						lang = parts[0]
						dir = parts[1]
					}
					triples = append(triples, p.tripleWithLang(subject, propURI, objectValue, lang, dir))
				} else if datatype != "" {
					triples = append(triples, p.tripleWithDatatype(subject, propURI, objectValue, datatype))
				} else {
					triples = append(triples, p.triple(subject, propURI, objectValue))
				}
			}

		case xml.EndElement:
			return triples, nil
		}
	}
}

func (p *XMLParser) parsePropertyContent(decoder *xml.Decoder, propEl xml.StartElement, subject, propURI string, depth int) ([]types.Triple, string, error) {
	var charData strings.Builder

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			return nil, charData.String(), nil
		}
		if err != nil {
			return nil, "", fmt.Errorf("xml parse error: %w", err)
		}

		switch t := tok.(type) {
		case xml.CharData:
			charData.Write(t)

		case xml.StartElement:
			innerTriples, err := p.parseDescription(decoder, t, depth+1)
			if err != nil {
				return nil, "", err
			}

			nestedSubject := p.getSubject(t)
			var allTriples []types.Triple
			allTriples = append(allTriples, p.triple(subject, propURI, nestedSubject))
			allTriples = append(allTriples, innerTriples...)

			if err := skipToEnd(decoder, propEl.Name); err != nil {
				return nil, "", err
			}

			return allTriples, "", nil

		case xml.EndElement:
			text := strings.TrimSpace(charData.String())
			return nil, text, nil
		}
	}
}

func (p *XMLParser) getSubject(el xml.StartElement) string {
	if about := getAttr(el, rdfNS, "about"); about != "" {
		return about
	}
	if about := getAttrLocal(el, "about"); about != "" {
		return about
	}
	if id := getAttr(el, rdfNS, "ID"); id != "" {
		return "#" + id
	}
	if nodeID := getAttr(el, rdfNS, "nodeID"); nodeID != "" {
		return "_:" + nodeID
	}
	return newBlankNode()
}

func (p *XMLParser) triple(s, pred, o string) types.Triple {
	return types.Triple{
		Subject:   s,
		Predicate: pred,
		Object:    o,
		Graph:     p.Graph,
	}
}

func (p *XMLParser) tripleWithLang(s, pred, o, lang, dir string) types.Triple {
	t := types.Triple{
		Subject:   s,
		Predicate: pred,
		Object:    o,
		Graph:     p.Graph,
	}
	if lang != "" {
		t.IsLiteral = true
		if dir != "" {
			t.Datatype = types.RDFDirLangString
			t.Direction = dir
		} else {
			t.Datatype = types.RDFLangString
		}
		t.Language = lang
	}
	return t
}

func (p *XMLParser) tripleWithDatatype(s, pred, o, datatype string) types.Triple {
	return types.Triple{
		Subject:   s,
		Predicate: pred,
		Object:    o,
		IsLiteral: true,
		Datatype:  datatype,
		Graph:     p.Graph,
	}
}

func getAttr(el xml.StartElement, ns, local string) string {
	for _, a := range el.Attr {
		if a.Name.Space == ns && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func getAttrLocal(el xml.StartElement, local string) string {
	for _, a := range el.Attr {
		if a.Name.Space == "" && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func skipElement(decoder *xml.Decoder) error {
	depth := 1
	for {
		tok, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("xml parse error while skipping: %w", err)
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				return nil
			}
		}
	}
}

func skipToEnd(decoder *xml.Decoder, name xml.Name) error {
	depth := 0
	for {
		tok, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("xml parse error while skipping to end: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			if depth == 0 && t.Name == name {
				return nil
			}
			depth--
		}
	}
}
