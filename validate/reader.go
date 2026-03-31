// Package validate provides SKOS quality validation functionality.
//
// This package offers tools to validate SKOS (Simple Knowledge Organization
// System) RDF vocabularies against quality criteria inspired by qSKOS.
// It supports both Turtle (.ttl) and RDF/XML (.rdf) file formats with
// automatic format detection.
//
// Example:
//
//	reader, err := validate.NewReader("vocabulary.ttl")
//	triples, err := reader.Parse()
//	validator := validate.NewValidator(triples)
//	report, err := validator.Validate(ctx)
package validate

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/soypete/ontology-go/rdf"
	"github.com/soypete/ontology-go/ttl"
	"github.com/soypete/ontology-go/types"
)

type Format int

const (
	FormatUnknown Format = iota
	FormatTTL
	FormatRDFXML
)

type Reader struct {
	filename string
	format   Format
	graph    string
}

func NewReader(filename string) *Reader {
	return &Reader{filename: filename, graph: "https://example.org/graph"}
}

func (r *Reader) SetGraph(graph string) {
	r.graph = graph
}

func (r *Reader) detectFormat() (Format, error) {
	f, err := os.Open(r.filename)
	if err != nil {
		return FormatUnknown, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lines := 0
	maxLines := 10

	var preview strings.Builder
	for scanner.Scan() && lines < maxLines {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			preview.WriteString(line)
			preview.WriteString("\n")
		}
		lines++
	}

	content := preview.String()
	contentLower := strings.ToLower(content)

	if strings.Contains(content, "@prefix") ||
		strings.Contains(contentLower, "prefix ") ||
		strings.HasPrefix(strings.TrimSpace(content), "@base") ||
		strings.HasPrefix(strings.TrimSpace(contentLower), "base ") {
		return FormatTTL, nil
	}

	if strings.Contains(content, "<?xml") ||
		strings.Contains(content, "<rdf:RDF") ||
		strings.Contains(content, "<rdf:Description") {
		return FormatRDFXML, nil
	}

	return FormatUnknown, fmt.Errorf("unable to detect format for file: %s", r.filename)
}

func (r *Reader) Parse() ([]types.Triple, error) {
	if r.format == FormatUnknown {
		format, err := r.detectFormat()
		if err != nil {
			return nil, err
		}
		r.format = format
	}

	var parser interface {
		Parse(io.Reader) ([]types.Triple, error)
	}

	switch r.format {
	case FormatTTL:
		p := ttl.NewTurtleParser()
		p.Graph = r.graph
		parser = p
	case FormatRDFXML:
		parser = rdf.NewXMLParser(r.graph)
	default:
		return nil, fmt.Errorf("unknown format: %v", r.format)
	}

	f, err := os.Open(r.filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	return parser.Parse(f)
}

func (r *Reader) Format() Format {
	if r.format == FormatUnknown {
		r.detectFormat()
	}
	return r.format
}

func ParseFile(filename string) ([]types.Triple, error) {
	r := NewReader(filename)
	return r.Parse()
}

func ParseFileWithGraph(filename, graph string) ([]types.Triple, error) {
	r := NewReader(filename)
	r.SetGraph(graph)
	return r.Parse()
}
