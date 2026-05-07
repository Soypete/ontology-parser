# Ontology Visualization Tool - PROGRESS

## Completed

### cmd/viz/main.go
- CLI entry point with `-http` flag
- File loading with owl:imports resolution and deduplication
- Uses `filepath.Abs`, `types.Triple`

### viz/graph.go
- Graph struct with Nodes, Edges, Namespaces, Types, Predicates
- ProcessTriples() extracts classes, properties, relationships
- Labels and comments via rdfs:label, rdfs:comment
- Namespace extraction and label extraction
- ToJSON() for serialization

### viz/server.go
- NewServer() loads files and creates graph
- HandleGraph() serves /api/graph endpoint
- EmbeddedFS() serves embedded HTML

### viz/index.html
- D3 force layout visualization
- Node type filtering (checkboxes)
- Relationship type filtering (checkboxes)
- Namespace filtering (checkboxes)
- Zoom/pan with d3.zoom
- Drag nodes with d3.drag

## Build & Run

```bash
go build -o ontology-viz ./cmd/viz/
./ontology-viz -http=:8080 testdata/skos.ttl testdata/edu.ttl
```

## Tested
- API endpoint `/api/graph` returns JSON with nodes/edges
- Successfully loaded testdata/skos.ttl