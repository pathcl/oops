package search

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/pathcl/oops/internal/parser"
)

const (
	k1            = 1.5
	b             = 0.75
	minScoreRatio = 0.3 // drop results scoring below 30% of the top result
)

// stopWords are removed from the query before scoring. They appear in section
// prose ("Show only...", "Find traces where...") and cause false matches.
var stopWords = map[string]bool{
	// query starters / command verbs
	"show": true, "find": true, "get": true, "list": true, "display": true,
	"give": true, "tell": true, "what": true, "how": true, "which": true,
	// common function words
	"me": true, "the": true, "an": true, "of": true, "to": true, "in": true,
	"is": true, "are": true, "was": true, "that": true, "where": true,
	"when": true, "with": true, "for": true, "on": true, "at": true,
	"by": true, "from": true, "as": true, "all": true, "only": true,
	// comparison words unlikely to appear in query content
	"than": true, "more": true, "less": true, "above": true, "below": true,
	"between": true, "over": true, "under": true,
}

// Result pairs a section with its relevance score.
type Result struct {
	Section parser.Section
	Score   float64
}

// Search ranks sections by BM25 relevance to the query and returns top n.
func Search(sections []parser.Section, query string, n int) []Result {
	if len(sections) == 0 || query == "" {
		return nil
	}

	queryTokens := tokenizeQuery(query)
	if len(queryTokens) == 0 {
		return nil
	}

	docs := make([][]string, len(sections))
	for i, s := range sections {
		docs[i] = tokenize(docText(s))
	}

	avgLen := avgDocLen(docs)
	idf := computeIDF(docs, queryTokens)

	type scored struct {
		idx   int
		score float64
	}
	var scored_ []scored
	for i, tokens := range docs {
		s := bm25Score(tokens, queryTokens, idf, avgLen)
		if s > 0 {
			scored_ = append(scored_, scored{i, s})
		}
	}

	sort.Slice(scored_, func(a, b int) bool {
		return scored_[a].score > scored_[b].score
	})

	if len(scored_) == 0 {
		return nil
	}

	threshold := scored_[0].score * minScoreRatio
	var filtered []struct{ idx int; score float64 }
	for _, s := range scored_ {
		if s.score >= threshold {
			filtered = append(filtered, s)
		}
	}

	if n > len(filtered) {
		n = len(filtered)
	}
	results := make([]Result, n)
	for i := range results {
		results[i] = Result{
			Section: sections[filtered[i].idx],
			Score:   filtered[i].score,
		}
	}
	return results
}

func docText(s parser.Section) string {
	return strings.Join([]string{s.Category, s.Title, s.Body, s.CodeBlock}, " ")
}

func tokenizeQuery(s string) []string {
	tokens := tokenize(s)
	out := tokens[:0]
	for _, t := range tokens {
		if !stopWords[t] {
			out = append(out, t)
		}
	}
	return out
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) > 1 {
			out = append(out, f)
		}
	}
	return out
}

func avgDocLen(docs [][]string) float64 {
	total := 0
	for _, d := range docs {
		total += len(d)
	}
	return float64(total) / float64(len(docs))
}

func computeIDF(docs [][]string, queryTokens []string) map[string]float64 {
	N := float64(len(docs))
	df := make(map[string]int)
	for _, tokens := range docs {
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
	}
	idf := make(map[string]float64, len(queryTokens))
	for _, t := range queryTokens {
		idf[t] = math.Log((N-float64(df[t])+0.5)/(float64(df[t])+0.5) + 1)
	}
	return idf
}

func bm25Score(docTokens, queryTokens []string, idf map[string]float64, avgLen float64) float64 {
	tf := make(map[string]float64)
	for _, t := range docTokens {
		tf[t]++
	}
	docLen := float64(len(docTokens))

	var score float64
	for _, qt := range queryTokens {
		f := tf[qt]
		numerator := f * (k1 + 1)
		denominator := f + k1*(1-b+b*docLen/avgLen)
		score += idf[qt] * (numerator / denominator)
	}
	return score
}
