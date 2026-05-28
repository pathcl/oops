package parser

import (
	"testing"
)

const sampleMarkdown = `# SRE Cheatsheet

## PromQL

### Rate of requests
Calculates per-second rate of HTTP requests over 5 minutes.
` + "```promql" + `
rate(http_requests_total[5m])
` + "```" + `

### Error ratio
Ratio of 5xx errors to total requests.
` + "```promql" + `
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
` + "```" + `

## LogQL

### Filter errors
Show only error-level log lines.
` + "```logql" + `
{app="myapp"} |= "error"
` + "```" + `

## TraceQL

### Slow traces
Find traces where root span duration exceeds 1 second.
` + "```traceql" + `
{ .http.status_code = 200 && duration > 1s }
` + "```"

func TestParse_SectionCount(t *testing.T) {
	sections := Parse(sampleMarkdown)
	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}
}

func TestParse_Categories(t *testing.T) {
	sections := Parse(sampleMarkdown)
	want := []string{"PromQL", "PromQL", "LogQL", "TraceQL"}
	for i, s := range sections {
		if s.Category != want[i] {
			t.Errorf("section %d: category = %q, want %q", i, s.Category, want[i])
		}
	}
}

func TestParse_Titles(t *testing.T) {
	sections := Parse(sampleMarkdown)
	want := []string{"Rate of requests", "Error ratio", "Filter errors", "Slow traces"}
	for i, s := range sections {
		if s.Title != want[i] {
			t.Errorf("section %d: title = %q, want %q", i, s.Title, want[i])
		}
	}
}

func TestParse_CodeBlocks(t *testing.T) {
	sections := Parse(sampleMarkdown)
	if sections[0].CodeBlock != "rate(http_requests_total[5m])" {
		t.Errorf("unexpected code block: %q", sections[0].CodeBlock)
	}
	if sections[0].Lang != "promql" {
		t.Errorf("unexpected lang: %q", sections[0].Lang)
	}
}

func TestParse_Body(t *testing.T) {
	sections := Parse(sampleMarkdown)
	if sections[0].Body == "" {
		t.Error("expected non-empty body for first section")
	}
}

func TestParse_Empty(t *testing.T) {
	sections := Parse("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty input, got %d", len(sections))
	}
}
