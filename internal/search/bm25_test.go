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

// --- Stemming tests ---

// TestStem verifies the stemmer reduces words to their expected root forms.
func TestStem(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"queries", "queri"},
		{"querying", "queri"},
		{"slower", "slower"}, // Porter keeps "slower" — "slow" is the base
		{"crashes", "crash"},
		{"crashed", "crash"},
		{"crashing", "crash"},
		{"requests", "request"},
		{"running", "run"},
		{"errors", "error"},
		{"latencies", "latenc"},
	}
	for _, c := range cases {
		got := stemWord(c.input)
		if got != c.want {
			t.Errorf("stemWord(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestSearch_Stemming_PluralQuery verifies that a plural query term matches
// a section whose content uses the singular form.
func TestSearch_Stemming_PluralQuery(t *testing.T) {
	sections := []parser.Section{
		{Category: "PromQL", Title: "Pod restart count", Body: "Number of container restarts in the last hour", CodeBlock: "increase(kube_pod_container_status_restarts_total[1h])", Lang: "promql"},
		{Category: "LogQL", Title: "Filter errors", Body: "Show only error log lines", CodeBlock: `{app="myapp"} |= "error"`, Lang: "logql"},
	}
	// "restarts" should stem to match "restart" in the section title/body
	results := Search(sections, "pod restarts", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result for stemmed query")
	}
	if results[0].Section.Title != "Pod restart count" {
		t.Errorf("expected 'Pod restart count' as top result, got %q", results[0].Section.Title)
	}
}

// TestSearch_Stemming_GerundQuery verifies that a gerund query term matches
// a section using the base verb form.
func TestSearch_Stemming_GerundQuery(t *testing.T) {
	sections := []parser.Section{
		{Category: "TraceQL", Title: "Slow traces", Body: "Find traces where duration exceeds 1 second", CodeBlock: "{ duration > 1s }", Lang: "traceql"},
		{Category: "PromQL", Title: "Error ratio", Body: "Ratio of 5xx errors", CodeBlock: "rate(errors[5m])", Lang: "promql"},
	}
	// "exceeding" should stem to match "exceed" in the section body
	results := Search(sections, "traces exceeding duration", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Title != "Slow traces" {
		t.Errorf("expected 'Slow traces' as top result, got %q", results[0].Section.Title)
	}
}

// --- Synonym tests ---

// TestSynonymExpansion verifies that query tokens are expanded with their
// domain synonyms before scoring.
func TestSynonymExpansion(t *testing.T) {
	// "response" is an SRE synonym for "latency"; tokens are already stemmed
	// when passed to expandSynonyms, so we compare against stemmed forms.
	wantStem := stemWord("latency")
	expanded := expandSynonyms([]string{stemWord("response")})
	found := false
	for _, tok := range expanded {
		if tok == wantStem {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stemmed 'latency' (%q) in synonym expansion of ['response'], got %v", wantStem, expanded)
	}
}

// TestSearch_Synonym_LatencyResponseTime verifies that querying "response time"
// finds a section that only uses the word "latency".
func TestSearch_Synonym_LatencyResponseTime(t *testing.T) {
	sections := []parser.Section{
		{Category: "PromQL", Title: "P99 latency by service", Body: "99th-percentile latency per service", CodeBlock: "histogram_quantile(0.99, ...)", Lang: "promql"},
		{Category: "LogQL", Title: "Filter errors", Body: "Show only error log lines", CodeBlock: `{app="myapp"} |= "error"`, Lang: "logql"},
	}
	results := Search(sections, "response time p99", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result for synonym query")
	}
	if results[0].Section.Title != "P99 latency by service" {
		t.Errorf("expected 'P99 latency by service' as top result, got %q", results[0].Section.Title)
	}
}

// TestSearch_Synonym_ContainerPod verifies that "container" matches a section
// that only uses "pod".
func TestSearch_Synonym_ContainerPod(t *testing.T) {
	sections := []parser.Section{
		{Category: "PromQL", Title: "Pod restart count", Body: "Number of pod restarts in the last hour detects crash loops", CodeBlock: "increase(kube_pod_container_status_restarts_total[1h])", Lang: "promql"},
		{Category: "TraceQL", Title: "Traces with errors", Body: "All spans in error state", CodeBlock: "{ status = error }", Lang: "traceql"},
	}
	results := Search(sections, "container crash restart", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Title != "Pod restart count" {
		t.Errorf("expected 'Pod restart count' as top result, got %q", results[0].Section.Title)
	}
}

// TestSearch_Synonym_ErrorFailure verifies that "failure" matches sections
// using the word "error".
func TestSearch_Synonym_ErrorFailure(t *testing.T) {
	sections := []parser.Section{
		{Category: "PromQL", Title: "Error ratio", Body: "Ratio of 5xx errors to total requests", CodeBlock: "rate(errors[5m])", Lang: "promql"},
		{Category: "TraceQL", Title: "Slow traces", Body: "Traces where duration exceeds 1 second", CodeBlock: "{ duration > 1s }", Lang: "traceql"},
	}
	results := Search(sections, "request failure ratio", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Title != "Error ratio" {
		t.Errorf("expected 'Error ratio' as top result, got %q", results[0].Section.Title)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
