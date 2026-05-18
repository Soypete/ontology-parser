# Ontology Visualization Tool

A CLI tool for visualizing RDF/OWL/Turtle ontologies in a web-based force-directed graph.

## Installation

### Prerequisites

- Go 1.24+
- Graphviz (optional, for dot export)

```bash
# macOS
brew install graphviz

# Linux
sudo apt-get install graphviz

# Windows
choco install graphviz
```

### Build

```bash
go build -o ontology-viz ./cmd/viz/
```

## Usage

```bash
ontology-viz -http=:8080 file1.ttl file2.ttl file3.ttl
```

Then open http://localhost:8080 in your browser.

### Options

- `-http` - HTTP server address (e.g., `:8080`, `localhost:8080`)

### Features

- **Multiple file support** - Load multiple .ttl files at once
- **owl:imports** - Automatically follows local owl:imports (skips remote)
- **Filtering** - Toggle node types, relationships, and namespaces
- **Zoom/Pan** - Mouse wheel to zoom, drag to pan
- **Drag nodes** - Move nodes around the graph
- **Hover** - See full IRI and details on hover

### Node Types

- class
- objectProperty
- datatypeProperty
- annotationProperty

### Relationship Types

- rdf:type
- rdfs:subClassOf
- rdfs:subPropertyOf
- owl:equivalentClass
- owl:equivalentProperty
- owl:inverseOf
- skos:broader
- skos:narrower
- skos:exactMatch
- skos:closeMatch

## Example

```bash
ontology-viz -http=:8080 \
  ./ontologies/edu.ttl \
  ./ontologies/sai.ttl \
  ./cv/canvas.ttl \
  ./cv/rostering.ttl \
  ./cv/schoology.ttl
```