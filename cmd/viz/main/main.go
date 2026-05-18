package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/soypete/ontology-go/cmd/viz"
	"github.com/soypete/ontology-go/ttl"
	"github.com/soypete/ontology-go/types"
)

var httpAddr = flag.String("http", "", "serve HTTP on this address (e.g., :8080)")

func main() {
	flag.Parse()

	if *httpAddr == "" {
		fmt.Println("Usage: ontology-viz [-http=:8080] file1.ttl [file2.ttl ...]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	files := flag.Args()
	if len(files) == 0 {
		fmt.Println("Error: at least one .ttl file required")
		os.Exit(1)
	}

	vizServer, err := viz.NewServer(files)
	if err != nil {
		log.Fatalf("Failed to load ontology: %v", err)
	}

	http.Handle("/", http.FileServer(vizServer.EmbeddedFS()))
	http.HandleFunc("/api/graph", vizServer.HandleGraph)

	log.Printf("Opening http://localhost%s in your browser...", *httpAddr)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

type Server struct {
	graph *viz.Graph
}

func NewServer(files []string) (*Server, error) {
	allTriples, err := loadFiles(files)
	if err != nil {
		return nil, err
	}

	g := viz.NewGraph()
	g.ProcessTriples(allTriples)

	return &Server{graph: g}, nil
}

func (s *Server) HandleGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.graph.ToJSON())
}

func (s *Server) EmbeddedFS() http.FileSystem {
	return http.Dir(".")
}

func loadFiles(paths []string) ([]types.Triple, error) {
	loaded := make(map[string]bool)
	var triples []types.Triple

	for _, path := range paths {
		t, err := loadWithImports(path, loaded)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", path, err)
		}
		triples = append(triples, t...)
	}

	return triples, nil
}

func loadWithImports(path string, loaded map[string]bool) ([]types.Triple, error) {
	absPath, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if loaded[absPath] {
		return nil, nil
	}
	loaded[absPath] = true

	parser := ttl.NewTurtleParser()
	triples, err := parser.ParseFile(absPath)
	if err != nil {
		return nil, err
	}

	var allTriples []types.Triple
	allTriples = append(allTriples, triples...)

	for _, t := range triples {
		if t.Predicate == types.OWLNS+"imports" && !t.IsLiteral {
			importPath := t.Object
			if strings.HasPrefix(importPath, "http://") || strings.HasPrefix(importPath, "https://") {
				fmt.Printf("Skipping remote import: %s\n", importPath)
				continue
			}
			importTriples, err := loadWithImports(importPath, loaded)
			if err != nil {
				return nil, fmt.Errorf("failed to load import %s: %w", importPath, err)
			}
			allTriples = append(allTriples, importTriples...)
		}
	}

	return allTriples, nil
}

func resolvePath(path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", err
	}

	return absPath, nil
}
