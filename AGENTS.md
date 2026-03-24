# AGENTS.md - Agent Coding Guidelines

This file provides guidance for AI agents working in this repository.

## Repository Overview

This is a Go library for parsing and analyzing RDF/OWL/TTL ontology files. The module is `github.com/soypete/ontology-go` with packages in the root directory: `rdf`, `store`, `sparql`, `ttl`, `fetch`, `types`.

## Workflow Guidelines

### Atomic Changes and Pull Requests

All changes should be atomic and submitted via pull requests:

1. **Create a feature branch** for each logical change:
   ```bash
   git checkout -b feature/add-sparql-support
   ```

2. **Make your changes** and commit with descriptive messages:
   ```bash
   git add <files>
   git commit -m "Add SPARQL query engine implementation"
   ```

3. **Push and create PR**:
   ```bash
   git push -u origin feature/add-sparql-support
   gh pr create --title "Add SPARQL query engine" --body "..."
   ```

4. **Wait for CI** to pass before requesting review

5. **Never push directly to main** - always use PRs

### Package Structure

```
/rdf/             # RDF/OWL parsing (XML format)
/store/           # Triple storage interfaces and implementations
/sparql/          # SPARQL query support
/ttl/             # Turtle format parsing
/fetch/           # Remote ontology fetching
/types/           # Core RDF type definitions
```

## TBOX/ABOX Mapping

This library supports mapping relational data to RDF triples using YAML configuration:

- **TBOX**: Schema-level mappings defining predicates and class types
- **ABOX**: Instance-level mappings generating individual triples from database rows

## Build, Lint, and Test Commands

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./rdf/...

# Run a single test function
go test -run TestXMLParser ./rdf/...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...
```

### Building

```bash
# Build the library
go build ./...

# Build specific packages
go build ./rdf/...
```

### Linting and Formatting

```bash
# Format code (use gofmt)
gofmt -w ./rdf/

# Check formatting without modifying
gofmt -d ./rdf/
```

### Building

```bash
# Build the library
go build ./...

# Build specific packages
go build ./pkg/rdf/...
go build ./pkg/rdf/examples/...
```

### Linting and Formatting

```bash
# Format code (use gofmt)
gofmt -w ./pkg/rdf/

# Check formatting without modifying
gofmt -d ./pkg/rdf/

# Run go vet
go vet ./...

# Run all checks (equivalent to CI)
go vet ./... && go test -race ./... && go build ./...
```

### Running Examples

```bash
# Run basic example
go run ./pkg/rdf/examples/basic/

# Run LLM example
go run ./pkg/rdf/examples/with_llm/
```

## Code Style Guidelines

### General Principles

- Use Go 1.24.7 or later
- Use `context.Context` for all operations that may be cancelled
- Use `slog` for structured logging
- Use functional options for configuration (see `WithStore`, `WithLogger`)
- Return concrete types where possible; use interfaces for abstraction

### Imports

Standard library imports first, then third-party imports, separated by a blank line:

```go
import (
    "context"
    "fmt"
    "io"
    "log/slog"

    "gopkg.in/yaml.v3"
)
```

### Naming Conventions

- **Packages**: lowercase, short, descriptive (e.g., `triplestore`, `mapping`)
- **Types**: PascalCase (e.g., `PKO`, `Triple`, `MemoryStore`)
- **Functions/Methods**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase
- **Constants**: PascalCase for exported, camelCase for unexported
- **Interfaces**: Name after the method they define + "er" suffix where natural (e.g., `Store`, `Enricher`, `Reader`)

### Error Handling

- Use `fmt.Errorf` with `%w` for error wrapping: `fmt.Errorf("failed to process: %w", err)`
- Return early on errors to avoid nested code
- Provide context in error messages (what failed, why)
- Use sentinel errors sparingly

```go
// Good
if err != nil {
    return fmt.Errorf("failed to load mapping: %w", err)
}

// Bad
if err != nil {
    return err
}
```

### Context Usage

Always accept `context.Context` as the first parameter for operations that may be slow or need cancellation:

```go
func (p *PKO) GenerateTriples(ctx context.Context) error
func (p *PKO) Export(ctx context.Context, format string, w io.Writer) error
```

Check for context cancellation periodically in long-running operations:

```go
select {
case <-ctx.Done():
    return ctx.Err()
default:
}
```

### Type Definitions

- Use structs for data types with multiple fields
- Use interfaces for abstraction points (e.g., `Store`, `TableReader`)
- Use concrete types for return values unless polymorphism is needed
- Embed interfaces rather than concrete types when appropriate

```go
// Good - interface for abstraction
type Store interface {
    Add(ctx context.Context, triples ...Triple) error
    Query(ctx context.Context, sparql string) ([]map[string]string, error)
}

// Good - concrete type for data
type Triple struct {
    Subject   string
    Predicate string
    Object    string
    IsLiteral bool
    Datatype  string
    Lang      string
}
```

### Testing Guidelines

- Use the standard `testing` package
- Test files should be named `*_test.go`
- Test functions should be named `Test<FunctionName>` or `Test<TypeName>/<Scenario>`
- Use table-driven tests for multiple scenarios

```go
func TestNewPKOFromConfig(t *testing.T) {
    // Test implementation
}

func TestPKO_GenerateTriples(t *testing.T) {
    tests := []struct {
        name    string
        config  *mapping.Config
        wantErr bool
    }{
        {"valid config", config, false},
        {"nil config", nil, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

### Comments

- Add package-level comments explaining the package purpose
- Document exported functions with full sentences (godoc style)
- Do NOT add comments for implementation details unless non-obvious

```go
// Package pkordf provides RDF triple generation from relational data.
type PKO struct { ... }

// NewPKO creates a new PKO processor from a mapping file.
func NewPKO(mappingPath string, reader db.TableReader, opts ...Option) (*PKO, error)
```

### Configuration and Options

Use the functional options pattern for optional configuration:

```go
type Option func(*PKO)

func WithStore(store triplestore.Store) Option {
    return func(p *PKO) {
        p.store = store
    }
}

func WithLogger(logger *slog.Logger) Option {
    return func(p *PKO) {
        p.logger = logger
    }
}

// Usage
pko, err := pkordf.NewPKOFromConfig(config, reader,
    pkordf.WithStore(customStore),
    pkordf.WithLogger(logger),
)
```

### Logging

- Use `slog` for structured logging
- Use appropriate log levels: Debug for verbose info, Info for normal operations, Warn for recoverable issues
- Include relevant fields in log statements

```go
p.logger.Info("triple generation complete", "count", p.store.Count())
p.logger.Debug("processing table", "table", tableName)
```