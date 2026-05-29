package llm

import (
	"context"
	"testing"

	"github.com/pathcl/oops/internal/parser"
	"github.com/pathcl/oops/internal/search"
)

// --- unit tests for helpers ---

func TestParseRanking_Valid(t *testing.T) {
	cases := []struct {
		input string
		want  []int
	}{
		{`{"ranking":[2,0,1]}`, []int{2, 0, 1}},
		{`{"ranking":[0]}`, []int{0}},
		{`{"ranking":[3,1,4,0,2]}`, []int{3, 1, 4, 0, 2}},
	}
	for _, c := range cases {
		got, err := parseRanking(c.input)
		if err != nil {
			t.Errorf("parseRanking(%q) unexpected error: %v", c.input, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("parseRanking(%q) len=%d, want %d", c.input, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("parseRanking(%q)[%d] = %d, want %d", c.input, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseRanking_Invalid(t *testing.T) {
	cases := []string{
		`{}`,
		`{"ranking":[]}`,
		`not json`,
	}
	for _, c := range cases {
		_, err := parseRanking(c)
		if err == nil {
			t.Errorf("parseRanking(%q) expected error, got nil", c)
		}
	}
}

func TestApplyRanking(t *testing.T) {
	sections := []parser.Section{
		{Title: "A"},
		{Title: "B"},
		{Title: "C"},
	}
	results := []search.Result{
		{Section: sections[0], Score: 1.0},
		{Section: sections[1], Score: 0.8},
		{Section: sections[2], Score: 0.5},
	}

	// ranking [2,0,1] means: 3rd result first, then 1st, then 2nd
	ranking := []int{2, 0, 1}
	got := applyRanking(results, ranking)

	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if got[0].Section.Title != "C" {
		t.Errorf("got[0] = %q, want C", got[0].Section.Title)
	}
	if got[1].Section.Title != "A" {
		t.Errorf("got[1] = %q, want A", got[1].Section.Title)
	}
	if got[2].Section.Title != "B" {
		t.Errorf("got[2] = %q, want B", got[2].Section.Title)
	}
}

func TestApplyRanking_OutOfBoundsSkipped(t *testing.T) {
	results := []search.Result{
		{Section: parser.Section{Title: "A"}},
		{Section: parser.Section{Title: "B"}},
	}
	// index 5 is out of bounds — should be skipped
	ranking := []int{1, 5, 0}
	got := applyRanking(results, ranking)
	if len(got) != 2 {
		t.Fatalf("expected 2 valid results, got %d", len(got))
	}
	if got[0].Section.Title != "B" {
		t.Errorf("got[0] = %q, want B", got[0].Section.Title)
	}
}

func TestBuildPrompt(t *testing.T) {
	results := []search.Result{
		{Section: parser.Section{Category: "PromQL", Title: "Error ratio", Body: "Ratio of errors.", CodeBlock: "rate(errors[5m])"}},
		{Section: parser.Section{Category: "LogQL", Title: "Filter errors", Body: "Show error logs.", CodeBlock: `{app="x"} |= "error"`}},
	}
	prompt := buildPrompt("show me error rate", results)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	// prompt must contain the query and section titles
	for _, want := range []string{"show me error rate", "Error ratio", "Filter errors", "PromQL", "LogQL"} {
		if !contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

// --- mock reranker for integration-style test ---

type mockReranker struct {
	ranking []int
	err     error
}

func (m *mockReranker) Rerank(_ context.Context, _ string, results []search.Result) ([]search.Result, Usage, error) {
	if m.err != nil {
		return nil, Usage{}, m.err
	}
	usage := Usage{InputTokens: 100, OutputTokens: 10, CostUSD: 0.0001}
	return applyRanking(results, m.ranking), usage, nil
}

func TestMockReranker_ReordersResults(t *testing.T) {
	results := []search.Result{
		{Section: parser.Section{Title: "A"}, Score: 1.0},
		{Section: parser.Section{Title: "B"}, Score: 0.9},
		{Section: parser.Section{Title: "C"}, Score: 0.8},
	}
	r := &mockReranker{ranking: []int{2, 0, 1}}
	got, usage, err := r.Rerank(context.Background(), "query", results)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Section.Title != "C" || got[1].Section.Title != "A" || got[2].Section.Title != "B" {
		t.Errorf("unexpected order: %v", titles(got))
	}
	if usage.InputTokens == 0 {
		t.Error("expected non-zero input tokens")
	}
}

func TestUsageString(t *testing.T) {
	u := Usage{InputTokens: 847, OutputTokens: 23, CostUSD: 0.0009635}
	s := u.String()
	if s == "" {
		t.Error("expected non-empty usage string")
	}
	// should contain token counts
	for _, want := range []string{"847", "23"} {
		if !contains(s, want) {
			t.Errorf("usage string missing %q: %s", want, s)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}

func titles(results []search.Result) []string {
	t := make([]string, len(results))
	for i, r := range results {
		t[i] = r.Section.Title
	}
	return t
}
