package search

import (
	"os"
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

// --- Absolute score threshold tests ---

// TestSearch_AbsoluteThreshold_UnrelatedQuery verifies that a query with no
// token overlap returns no results (already worked before the threshold).
func TestSearch_AbsoluteThreshold_UnrelatedQuery(t *testing.T) {
	results := Search(testSections, "pizza margherita recipe", 5)
	if len(results) != 0 {
		t.Errorf("expected no results for unrelated query, got %d", len(results))
	}
}

// TestSearch_AbsoluteThreshold_SingleWeakSynonym verifies that a query whose
// only connection to any section is one tangential synonym match returns no
// results on a realistic corpus — the match is noise, not signal.
// This test loads the real cheatsheet so that IDF values reflect a realistic
// corpus size; the bug only manifests with 70+ sections.
func TestSearch_AbsoluteThreshold_SingleWeakSynonym(t *testing.T) {
	data, err := os.ReadFile("../../testdata/cheatsheet.md")
	if err != nil {
		t.Skip("testdata/cheatsheet.md not found — skipping corpus-dependent test")
	}
	sections := parser.Parse(string(data))

	// "throughput" in the query reaches "Request rate by service" via its body text,
	// but "network" and "bandwidth" have no overlap with anything.
	// A single tangential synonym hit on a 78-section corpus should be below threshold.
	results := Search(sections, "network bandwidth throughput", 5)
	if len(results) != 0 {
		t.Errorf("expected no results for tangential query, got %d (top: %q, score: %.3f)",
			len(results), results[0].Section.Title, results[0].Score)
	}
}

// TestSearch_AbsoluteThreshold_StrongMatch verifies that a genuine multi-token
// match still returns results after the threshold is applied.
func TestSearch_AbsoluteThreshold_StrongMatch(t *testing.T) {
	results := Search(testSections, "http request rate per second", 5)
	if len(results) == 0 {
		t.Error("expected results for strong query, got none after threshold")
	}
}

// TestSearch_AbsoluteThreshold_DirectTitleMatch verifies that an exact title
// word match returns results.
func TestSearch_AbsoluteThreshold_DirectTitleMatch(t *testing.T) {
	results := Search(testSections, "slow traces duration", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result for direct title match")
	}
	if results[0].Section.Title != "Slow traces" {
		t.Errorf("expected 'Slow traces' as top result, got %q", results[0].Section.Title)
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

// --- Category detection tests ---

// TestDetectCategory_Logs verifies that log-related keywords map to LogQL.
func TestDetectCategory_Logs(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"find error logs", "LogQL"},
		{"log lines with errors", "LogQL"},
		{"logql filter namespace", "LogQL"},
		{"loki query errors", "LogQL"},
	}
	for _, c := range cases {
		tokens := tokenizeQuery(c.query)
		got := detectCategory(tokens)
		if got != c.want {
			t.Errorf("detectCategory(%q tokens) = %q, want %q", c.query, got, c.want)
		}
	}
}

// TestDetectCategory_Metrics verifies that metric-related keywords map to PromQL.
func TestDetectCategory_Metrics(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"p99 latency metric", "PromQL"},
		{"prometheus counter rate", "PromQL"},
		{"histogram quantile cpu", "PromQL"},
		{"promql error rate", "PromQL"},
	}
	for _, c := range cases {
		tokens := tokenizeQuery(c.query)
		got := detectCategory(tokens)
		if got != c.want {
			t.Errorf("detectCategory(%q tokens) = %q, want %q", c.query, got, c.want)
		}
	}
}

// TestDetectCategory_Traces verifies that trace-related keywords map to TraceQL.
func TestDetectCategory_Traces(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"slow traces span duration", "TraceQL"},
		{"traceql error spans", "TraceQL"},
		{"distributed tracing service", "TraceQL"},
		{"tempo span attributes", "TraceQL"},
	}
	for _, c := range cases {
		tokens := tokenizeQuery(c.query)
		got := detectCategory(tokens)
		if got != c.want {
			t.Errorf("detectCategory(%q tokens) = %q, want %q", c.query, got, c.want)
		}
	}
}

// TestDetectCategory_Ambiguous verifies that generic queries return no category.
func TestDetectCategory_Ambiguous(t *testing.T) {
	cases := []string{"error rate", "high cpu", "restart count"}
	for _, q := range cases {
		tokens := tokenizeQuery(q)
		got := detectCategory(tokens)
		if got != "" {
			t.Errorf("detectCategory(%q tokens) = %q, want empty for ambiguous query", q, got)
		}
	}
}

// TestSearch_CategoryBoost_Logs verifies that "error logs" returns LogQL first.
func TestSearch_CategoryBoost_Logs(t *testing.T) {
	sections := []parser.Section{
		{Category: "LogQL", Title: "Error log rate per service", Body: "Rate of error-level log lines per service", CodeBlock: `sum by (service) (rate({namespace="production"} | json | level="error" [1m]))`, Lang: "logql"},
		{Category: "TraceQL", Title: "Timeout exceptions across services", Body: "Spans that recorded a TimeoutError exception", CodeBlock: `{ event.exception.type = "TimeoutError" }`, Lang: "traceql"},
		{Category: "PromQL", Title: "Error ratio", Body: "Ratio of 5xx errors to total requests", CodeBlock: "rate(errors[5m])", Lang: "promql"},
	}
	results := Search(sections, "find error logs", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Category != "LogQL" {
		t.Errorf("expected LogQL category first for 'find error logs', got %q (%s)", results[0].Section.Category, results[0].Section.Title)
	}
}

// TestSearch_CategoryBoost_Metrics verifies that metric queries return PromQL first.
func TestSearch_CategoryBoost_Metrics(t *testing.T) {
	sections := []parser.Section{
		{Category: "PromQL", Title: "P99 latency by service", Body: "99th-percentile latency metric per service", CodeBlock: "histogram_quantile(0.99, ...)", Lang: "promql"},
		{Category: "TraceQL", Title: "Slow traces", Body: "Traces where span duration exceeds 1 second", CodeBlock: "{ duration > 1s }", Lang: "traceql"},
		{Category: "LogQL", Title: "Slow requests from access logs", Body: "Requests taking longer than 1000ms", CodeBlock: `{app="api"} | json | duration_ms > 1000`, Lang: "logql"},
	}
	results := Search(sections, "p99 latency metric histogram", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Category != "PromQL" {
		t.Errorf("expected PromQL category first, got %q (%s)", results[0].Section.Category, results[0].Section.Title)
	}
}

// TestSearch_CategoryBoost_Traces verifies that trace queries return TraceQL first.
func TestSearch_CategoryBoost_Traces(t *testing.T) {
	sections := []parser.Section{
		{Category: "TraceQL", Title: "Traces with errors", Body: "All spans in error state across services", CodeBlock: "{ status = error }", Lang: "traceql"},
		{Category: "LogQL", Title: "Fatal errors with stack traces", Body: "Fatal log lines that include a stacktrace field", CodeBlock: `{namespace="production"} | json | level="fatal"`, Lang: "logql"},
		{Category: "PromQL", Title: "Error ratio", Body: "Ratio of 5xx errors", CodeBlock: "rate(errors[5m])", Lang: "promql"},
	}
	results := Search(sections, "error spans traces", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Section.Category != "TraceQL" {
		t.Errorf("expected TraceQL category first, got %q (%s)", results[0].Section.Category, results[0].Section.Title)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
