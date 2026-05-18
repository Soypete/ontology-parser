package viz

import (
	"embed"
	"fmt"
	"net/http"
	"strings"

	"github.com/soypete/ontology-go/ttl"
	"github.com/soypete/ontology-go/types"
)

//go:embed index.html
var HTML embed.FS

type Server struct {
	graph *Graph
}

func NewServer(files []string) (*Server, error) {
	loaded := make(map[string]bool)
	var triples []types.Triple

	for _, path := range files {
		t, err := loadWithImports(path, loaded)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", path, err)
		}
		triples = append(triples, t...)
	}

	graph := NewGraph()
	graph.ProcessTriples(triples)

	return &Server{graph: graph}, nil
}

func loadWithImports(path string, loaded map[string]bool) ([]types.Triple, error) {
	parser := ttl.NewTurtleParser()
	triples, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}

	if loaded[path] {
		return nil, nil
	}
	loaded[path] = true

	var allTriples []types.Triple
	allTriples = append(allTriples, triples...)

	for _, t := range triples {
		if t.Predicate == types.OWLNS+"imports" && !t.IsLiteral {
			importPath := t.Object
			if strings.HasPrefix(importPath, "http://") || strings.HasPrefix(importPath, "https://") {
				continue
			}
			importTriples, err := loadWithImports(importPath, loaded)
			if err != nil {
				return nil, fmt.Errorf("failed to load import %s: %w", t.Object, err)
			}
			allTriples = append(allTriples, importTriples...)
		}
	}

	return allTriples, nil
}

func (s *Server) HandleGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.graph.ToJSON())
}

func (s *Server) EmbeddedFS() http.FileSystem {
	return http.FS(HTML)
}
