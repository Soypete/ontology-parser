package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/soypete/ontology-go/site"
)

func main() {
	outputDir := flag.String("o", "./docs", "Output directory for generated site")
	title := flag.String("title", "Ontology", "Site title")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s site [flags] file...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGenerate static HTML documentation for ontology files.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSupported file formats: .ttl (Turtle)\n")
	}

	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: at least one file required")
		flag.Usage()
		os.Exit(1)
	}

	files := flag.Args()
	ctx := context.Background()

	renderer := site.NewRenderer(site.Config{
		OutputDir: *outputDir,
		Title:     *title,
	})

	if err := renderer.LoadOntology(ctx, files); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading ontology: %v\n", err)
		os.Exit(1)
	}

	if err := renderer.Render(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering site: %v\n", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(*outputDir)
	fmt.Printf("Site generated successfully in: %s\n", absPath)
}
