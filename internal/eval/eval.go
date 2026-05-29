package eval

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pathcl/oops/internal/llm"
	"github.com/pathcl/oops/internal/parser"
	"github.com/pathcl/oops/internal/search"
)

// QueryCase is one ground-truth entry from the eval file.
type QueryCase struct {
	Query      string `yaml:"query"`
	Expected   string `yaml:"expected"` // exact section title
	Difficulty string `yaml:"difficulty"`
}

type evalFile struct {
	Queries []QueryCase `yaml:"queries"`
}

// QueryResult holds per-query metrics for one system (BM25 or BM25+LLM).
type QueryResult struct {
	Query      string
	Expected   string
	Difficulty string
	TopTitle   string
	P1         float64 // Precision@1
	RR         float64 // Reciprocal Rank
}

// Report holds the side-by-side comparison of two systems.
type Report struct {
	BM25    []QueryResult
	LLM     []QueryResult
	LLMCost float64 // total USD spent on LLM calls
}

// LoadQueries reads the ground-truth YAML file.
func LoadQueries(path string) ([]QueryCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading eval file: %w", err)
	}
	var ef evalFile
	if err := yaml.Unmarshal(data, &ef); err != nil {
		return nil, fmt.Errorf("parsing eval file: %w", err)
	}
	return ef.Queries, nil
}

// Run evaluates BM25 alone and BM25+LLM against the ground-truth cases.
// reranker may be nil to skip the LLM column.
func Run(ctx context.Context, cases []QueryCase, sections []parser.Section, reranker llm.Reranker) Report {
	var report Report
	const candidates = 10
	const topN = 5

	for _, c := range cases {
		// BM25 only
		bm25Results := search.Search(sections, c.Query, topN)
		report.BM25 = append(report.BM25, QueryResult{
			Query:      c.Query,
			Expected:   c.Expected,
			Difficulty: c.Difficulty,
			TopTitle:   topTitle(bm25Results),
			P1:         precisionAt1(bm25Results, c.Expected),
			RR:         recipRank(bm25Results, c.Expected),
		})

		// BM25 + LLM
		if reranker != nil {
			candidates_ := search.Search(sections, c.Query, candidates)
			reranked, usage, err := reranker.Rerank(ctx, c.Query, candidates_)
			if err == nil && len(reranked) > topN {
				reranked = reranked[:topN]
			}
			if err != nil {
				reranked = bm25Results
			}
			report.LLMCost += usage.CostUSD
			report.LLM = append(report.LLM, QueryResult{
				Query:      c.Query,
				Expected:   c.Expected,
				Difficulty: c.Difficulty,
				TopTitle:   topTitle(reranked),
				P1:         precisionAt1(reranked, c.Expected),
				RR:         recipRank(reranked, c.Expected),
			})
		}
	}
	return report
}

// Print writes the report as a human-readable table to w.
func Print(w *os.File, r Report) {
	easy, hard := splitByDifficulty(r.BM25)
	fmt.Fprintf(w, "\n%-52s  %-6s  %-6s\n", "Query", "BM25", "")
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 68))

	for i, q := range r.BM25 {
		hit := "✗"
		if q.P1 == 1.0 {
			hit = "✓"
		}
		bm25Hit := hit

		llmHit := ""
		if len(r.LLM) > i {
			lq := r.LLM[i]
			if lq.P1 == 1.0 {
				llmHit = "✓"
			} else {
				llmHit = "✗"
			}
		}

		query := q.Query
		if len(query) > 48 {
			query = query[:45] + "..."
		}
		diff := q.Difficulty
		if diff == "hard" {
			diff = "HARD"
		} else {
			diff = "easy"
		}
		fmt.Fprintf(w, "%-4s %-48s  BM25:%-2s  LLM:%-2s\n", diff, query, bm25Hit, llmHit)
	}

	fmt.Fprintf(w, "\n%s\n", strings.Repeat("─", 68))
	fmt.Fprintf(w, "%-20s  %6s  %6s  %6s  %6s\n", "Metric", "BM25", "LLM", "Δ all", "")
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 68))

	printMetrics(w, "P@1 (all)", r.BM25, r.LLM)
	printMetrics(w, "MRR (all)", r.BM25, r.LLM)
	if len(easy) > 0 {
		easyLLM, _ := splitByDifficulty(r.LLM)
		printMetrics(w, "P@1 (easy)", easy, easyLLM)
	}
	if len(hard) > 0 {
		_, hardLLM := splitByDifficulty(r.LLM)
		printMetrics(w, "P@1 (hard)", hard, hardLLM)
		printMetrics(w, "MRR (hard)", hard, hardLLM)
	}

	if r.LLMCost > 0 {
		fmt.Fprintf(w, "\nTotal LLM cost: $%.4f for %d queries (~$%.4f/query)\n",
			r.LLMCost, len(r.LLM), r.LLMCost/float64(len(r.LLM)))
	}
}

func printMetrics(w *os.File, label string, bm25, llmRes []QueryResult) {
	b := avgMetric(bm25, func(q QueryResult) float64 { return q.P1 })
	if strings.Contains(label, "MRR") {
		b = mrr(bm25)
	}
	if len(llmRes) == 0 {
		fmt.Fprintf(w, "%-20s  %6.3f\n", label, b)
		return
	}
	l := avgMetric(llmRes, func(q QueryResult) float64 { return q.P1 })
	if strings.Contains(label, "MRR") {
		l = mrr(llmRes)
	}
	delta := l - b
	arrow := "+"
	if delta < 0 {
		arrow = ""
	}
	fmt.Fprintf(w, "%-20s  %6.3f  %6.3f  %s%.3f\n", label, b, l, arrow, delta)
}

// --- metric helpers ---

func precisionAt1(results []search.Result, expected string) float64 {
	if len(results) > 0 && results[0].Section.Title == expected {
		return 1.0
	}
	return 0.0
}

func recipRank(results []search.Result, expected string) float64 {
	for i, r := range results {
		if r.Section.Title == expected {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

func mrr(results []QueryResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		sum += r.RR
	}
	return sum / float64(len(results))
}

func avgMetric(results []QueryResult, f func(QueryResult) float64) float64 {
	if len(results) == 0 {
		return 0
	}
	var sum float64
	for _, r := range results {
		sum += f(r)
	}
	return sum / float64(len(results))
}

func splitByDifficulty(results []QueryResult) (easy, hard []QueryResult) {
	for _, r := range results {
		if r.Difficulty == "hard" {
			hard = append(hard, r)
		} else {
			easy = append(easy, r)
		}
	}
	return
}

func topTitle(results []search.Result) string {
	if len(results) == 0 {
		return ""
	}
	return results[0].Section.Title
}
