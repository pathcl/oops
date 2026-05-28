package search

import (
	"testing"

	"github.com/pathcl/oops/internal/parser"
)

var testSections = []parser.Section{
	{Category: "PromQL", Title: "Rate of requests", Body: "Per-second HTTP request rate", CodeBlock: "rate(http_requests_total[5m])", Lang: "promql"},
	{Category: "PromQL", Title: "Error ratio", Body: "Ratio of 5xx errors to total requests", CodeBlock: `sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))`, Lang: "promql"},
	{Category: "LogQL", Title: "Filter errors", Body: "Show only error-level log lines", CodeBlock: `{app="myapp"} |= "error"`, Lang: "logql"},
	{Category: "TraceQL", Title: "Slow traces", Body: "Find traces where duration exceeds 1 second", CodeBlock: `{ .http.status_code = 200 && duration > 1s }`, Lang: "traceql"},
}

func TestSearch_ReturnsTopN(t *testing.T) {
	results := Search(testSections, "error rate", 2)
	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestSearch_ErrorQuery(t *testing.T) {
	results := Search(testSections, "error logs filter", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	// LogQL "Filter errors" should rank highly for "error logs filter"
	found := false
	for _, r := range results[:min(2, len(results))] {
		if r.Section.Category == "LogQL" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LogQL section in top 2 results for 'error logs filter', got: %v", results[0].Section.Title)
	}
}

func TestSearch_RateQuery(t *testing.T) {
	results := Search(testSections, "http request rate per second", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Title != "Rate of requests" {
		t.Errorf("expected 'Rate of requests' as top result, got %q", results[0].Section.Title)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	results := Search(testSections, "", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestSearch_NoSections(t *testing.T) {
	results := Search(nil, "error rate", 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil sections, got %d", len(results))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
