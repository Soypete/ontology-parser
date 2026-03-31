// Package main provides a command-line tool for validating SKOS vocabularies.
//
// This tool uses only the Go standard library (flag, os, fmt, encoding/json)
// to avoid external dependencies and reduce supply chain attack risk.
//
// Usage:
//
//	ontology-go validate [flags] file...
//
// Examples:
//
//	# Validate a single file
//	ontology-go validate vocabulary.ttl
//
//	# Validate multiple files
//	ontology-go validate scheme1.ttl scheme2.rdf
//
//	# Output as JSON
//	ontology-go validate --format json vocabulary.ttl
//
//	# Only show errors
//	ontology-go validate --errors-only vocabulary.ttl
//
//	# Filter by minimum severity
//	ontology-go validate --severity error vocabulary.ttl
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/soypete/ontology-go/validate"
)

var (
	format     = flag.String("format", "text", "Output format: text, json")
	severity   = flag.String("severity", "info", "Minimum severity to show: info, warning, error")
	errorsOnly = flag.Bool("errors-only", false, "Only show errors (equivalent to --severity error)")
	quiet      = flag.Bool("quiet", false, "Suppress non-error output")
	verbose    = flag.Bool("verbose", false, "Show additional details like context and confidence")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s validate [flags] file...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nValidate SKOS vocabulary files.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSupported file formats: .ttl (Turtle), .rdf (RDF/XML)\n")
		fmt.Fprintf(os.Stderr, "Format is auto-detected based on file content.\n")
		fmt.Fprintf(os.Stderr, "\nUse --verbose to show additional details like context and confidence.\n")
	}

	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: at least one file required")
		flag.Usage()
		os.Exit(1)
	}

	minSeverity := *severity
	if *errorsOnly {
		minSeverity = "error"
	}

	files := flag.Args()
	ctx := context.Background()

	var allReports []validateFileResult
	hasErrors := false

	for _, filename := range files {
		result, err := validateFile(ctx, filename, minSeverity)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error validating %s: %v\n", filename, err)
			hasErrors = true
			continue
		}
		allReports = append(allReports, result)
	}

	if *format == "json" {
		outputJSON(allReports, minSeverity)
	} else {
		outputText(allReports, minSeverity, hasErrors)
	}

	if hasErrors {
		os.Exit(1)
	}
}

type validateFileResult struct {
	Filename string                     `json:"filename"`
	Format   string                     `json:"format"`
	Report   *validate.ValidationReport `json:"report,omitempty"`
	Error    string                     `json:"error,omitempty"`
}

func validateFile(ctx context.Context, filename, minSeverity string) (validateFileResult, error) {
	result := validateFileResult{Filename: filename}

	formatStr, err := detectFileFormat(filename)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.Format = formatStr

	triples, err := validate.ParseFile(filename)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	validator := validate.NewValidator(triples)
	report, err := validator.Validate(ctx)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Report = filterReport(report, minSeverity)
	return result, nil
}

func detectFileFormat(filename string) (string, error) {
	reader := validate.NewReader(filename)
	format := reader.Format()
	switch format {
	case validate.FormatTTL:
		return "Turtle", nil
	case validate.FormatRDFXML:
		return "RDF/XML", nil
	default:
		return "unknown", fmt.Errorf("unable to detect format for %s", filename)
	}
}

func filterReport(report *validate.ValidationReport, minSeverity string) *validate.ValidationReport {
	minSev := severityLevel(minSeverity)
	filtered := &validate.ValidationReport{
		TotalTriples:  report.TotalTriples,
		TotalConcepts: report.TotalConcepts,
		TotalSchemes:  report.TotalSchemes,
		Issues:        []validate.Issue{},
		Stats:         make(map[string]int),
	}

	for _, issue := range report.Issues {
		if severityLevel(string(issue.Severity)) >= minSev {
			filtered.Issues = append(filtered.Issues, issue)
			filtered.Stats[string(issue.Type)]++
		}
	}

	return filtered
}

func severityLevel(s string) int {
	switch s {
	case "info":
		return 0
	case "warning":
		return 1
	case "error":
		return 2
	default:
		return 0
	}
}

func outputJSON(reports []validateFileResult, minSeverity string) {
	minSev := severityLevel(minSeverity)

	type jsonOutput struct {
		Files []validateFileResult `json:"files"`
	}

	var filteredOutput jsonOutput
	for _, r := range reports {
		if r.Report != nil {
			filteredReport := validate.ValidationReport{
				TotalTriples:  r.Report.TotalTriples,
				TotalConcepts: r.Report.TotalConcepts,
				TotalSchemes:  r.Report.TotalSchemes,
				Issues:        []validate.Issue{},
				Stats:         make(map[string]int),
			}
			for _, issue := range r.Report.Issues {
				if severityLevel(string(issue.Severity)) >= minSev {
					filteredReport.Issues = append(filteredReport.Issues, issue)
					filteredReport.Stats[string(issue.Type)]++
				}
			}
			r.Report = &filteredReport
		}
		filteredOutput.Files = append(filteredOutput.Files, r)
	}

	data, err := json.MarshalIndent(filteredOutput, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func outputText(reports []validateFileResult, minSeverity string, hasErrors bool) {
	minSev := severityLevel(minSeverity)
	hasIssues := false

	for _, result := range reports {
		if result.Error != "" {
			fmt.Printf("✗ %s: %s\n", result.Filename, result.Error)
			continue
		}

		fmt.Printf("✓ %s (%s)\n", result.Filename, result.Format)
		fmt.Printf("  Triples: %d | Concepts: %d | Schemes: %d\n",
			result.Report.TotalTriples, result.Report.TotalConcepts, result.Report.TotalSchemes)

		if len(result.Report.Issues) == 0 {
			if !*quiet {
				fmt.Println("  No issues found")
			}
			continue
		}

		hasIssues = true

		sort.Slice(result.Report.Issues, func(i, j int) bool {
			if result.Report.Issues[i].Severity != result.Report.Issues[j].Severity {
				return severityLevel(string(result.Report.Issues[i].Severity)) > severityLevel(string(result.Report.Issues[j].Severity))
			}
			return result.Report.Issues[i].Type < result.Report.Issues[j].Type
		})

		for _, issue := range result.Report.Issues {
			sev := severityLevel(string(issue.Severity))
			if sev < minSev {
				continue
			}

			icon := "ℹ"
			if issue.Severity == validate.SeverityWarning {
				icon = "⚠"
			} else if issue.Severity == validate.SeverityError {
				icon = "✗"
			}

			fmt.Printf("  %s [%s] %s\n", icon, issue.Severity, issue.Message)
			fmt.Printf("      Subject: %s\n", issue.Subject)
			if issue.Type != "" {
				fmt.Printf("      Type: %s\n", issue.Type)
			}
			if *verbose {
				if issue.Context != nil {
					fmt.Printf("      Context: %v\n", issue.Context)
				}
				fmt.Printf("      Confidence: %.0f%%\n", issue.Confidence*100)
			}
		}

		fmt.Printf("\n  Summary: %d issues\n", len(result.Report.Issues))
		for issueType, count := range result.Report.Stats {
			fmt.Printf("    - %s: %d\n", issueType, count)
		}
		fmt.Println()
	}

	if !*quiet && !hasIssues && !hasErrors {
		fmt.Println("All files validated successfully.")
	}
}
