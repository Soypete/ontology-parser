package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/soypete/ontology-go/cmd/viz"
	"github.com/soypete/ontology-go/site"
	queryengine "github.com/soypete/ontology-go/sparql"
	"github.com/soypete/ontology-go/store"
)

var (
	port   = flag.String("p", "8080", "HTTP port to listen on")
	output = flag.String("o", "./.ontosite", "Temp directory for generated site")
)

type Server struct {
	renderer    *site.Renderer
	graph       *viz.Graph
	queryEngine *queryengine.Engine
	outputDir   string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s serve [flags] file...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nServe ontology documentation with SSR.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: at least one file required")
		flag.Usage()
		os.Exit(1)
	}

	files := flag.Args()
	ctx := context.Background()

	server, err := NewServer(ctx, files, *output)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	http.Handle("/", http.FileServer(http.Dir(server.outputDir)))
	http.HandleFunc("/api/graph", server.handleGraph)
	http.HandleFunc("/api/search", server.handleSearch)
	http.HandleFunc("/api/sparql", server.handleSPARQL)

	addr := ":" + *port
	log.Printf("Server starting at http://localhost%s", addr)
	log.Printf("Serving from: %s", server.outputDir)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func NewServer(ctx context.Context, files []string, outputDir string) (*Server, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	renderer := site.NewRenderer(site.Config{
		OutputDir: outputDir,
		Title:     "Ontology",
	})

	if err := renderer.LoadOntology(ctx, files); err != nil {
		return nil, fmt.Errorf("failed to load ontology: %w", err)
	}

	if err := renderer.Render(ctx); err != nil {
		return nil, fmt.Errorf("failed to render site: %w", err)
	}

	triples := renderer.Triples()
	g := viz.NewGraph()
	g.ProcessTriples(triples)

	memStore := store.NewMemoryStore()
	memStore.Register("default", triples)
	engine := queryengine.NewEngine(memStore)

	return &Server{
		renderer:    renderer,
		graph:       g,
		queryEngine: engine,
		outputDir:   outputDir,
	}, nil
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.graph.ToJSON())
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "missing q parameter", http.StatusBadRequest)
		return
	}

	classes := s.renderer.Classes()
	properties := s.renderer.Properties()

	queryLower := strings.ToLower(query)
	var results struct {
		Classes    []site.ClassInfo    `json:"classes"`
		Properties []site.PropertyInfo `json:"properties"`
	}

	for _, c := range classes {
		if strings.Contains(strings.ToLower(c.Name), queryLower) ||
			strings.Contains(strings.ToLower(c.Label), queryLower) {
			results.Classes = append(results.Classes, c)
		}
	}

	for _, p := range properties {
		if strings.Contains(strings.ToLower(p.Name), queryLower) ||
			strings.Contains(strings.ToLower(p.Label), queryLower) {
			results.Properties = append(results.Properties, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleSPARQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var query string
	if r.Method == http.MethodPost {
		r.ParseForm()
		query = r.Form.Get("query")
	} else {
		query = r.URL.Query().Get("query")
	}

	if query == "" {
		http.Error(w, "missing query parameter", http.StatusBadRequest)
		return
	}

	result, err := s.queryEngine.Execute(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("SPARQL error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
