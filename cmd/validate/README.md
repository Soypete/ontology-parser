# SKOS Validator CLI

A command-line tool for validating SKOS (Simple Knowledge Organization System) vocabularies.

## Installation

```bash
go build -o ontology-validate ./cmd/validate/
```

## Usage

```bash
ontology-validate [flags] file...
```

### Flags

- `--format` - Output format: `text` (default) or `json`
- `--severity` - Minimum severity to show: `info` (default), `warning`, `error`
- `--errors-only` - Only show errors (equivalent to `--severity error`)
- `--quiet` - Suppress non-error output
- `--verbose` - Show additional details like context and confidence

### Examples

Validate a single file:
```bash
ontology-validate vocabulary.ttl
```

Validate multiple files:
```bash
ontology-validate scheme1.ttl scheme2.rdf
```

Output as JSON:
```bash
ontology-validate --format json vocabulary.ttl
```

Only show errors:
```bash
ontology-validate --errors-only vocabulary.ttl
```

Filter by minimum severity:
```bash
ontology-validate --severity error vocabulary.ttl
```

## Supported File Formats

- `.ttl` - Turtle format
- `.rdf` - RDF/XML format

The file format is automatically detected based on file content.

## Design Decision

This CLI uses only Go standard library packages (`flag`, `os`, `fmt`, `encoding/json`) rather than external CLI libraries to avoid supply chain attack risks.