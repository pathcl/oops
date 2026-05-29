package search

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/kljensen/snowball"
	"github.com/pathcl/oops/internal/parser"
)

const (
	k1            = 1.5
	b             = 0.75
	minScoreRatio = 0.3 // drop results scoring below 30% of the top result
	categoryBoost = 2.5 // score multiplier for sections matching the detected category
	// minTokenCoverage is the minimum fraction of original (pre-synonym) query
	// tokens that must appear in at least one document. Below this the results
	// are coincidental and suppressed. Corpus-size independent.
	minTokenCoverage = 0.34
)

// categoryKeywords maps a category name to the stemmed query tokens that imply it.
var categoryKeywords = map[string][]string{
	"LogQL":   {"log", "logql", "loki", "loglin"},
	"PromQL":  {"metric", "promql", "promethe", "counter", "gaug", "histogram", "quantil"},
	"TraceQL": {"trace", "traceql", "span", "tempo", "distribut"},
}

// detectCategory returns the query language implied by the token set, or "" if ambiguous.
func detectCategory(tokens []string) string {
	tokenSet := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		tokenSet[t] = true
	}
	for cat, keywords := range categoryKeywords {
		for _, kw := range keywords {
			if tokenSet[kw] {
				return cat
			}
		}
	}
	return ""
}

// rawSynonyms is the human-readable synonym table. Keys and values are plain
// English; they are pre-stemmed into sreSynonyms at init time so that lookups
// always use the same stemmed form as tokenized query tokens.
var rawSynonyms = map[string][]string{
	"latency":    {"duration", "response", "delay", "slowness"},
	"duration":   {"latency", "response", "delay"},
	"response":   {"latency", "duration"},
	"error":      {"failure", "fault", "exception"},
	"failure":    {"error", "fault", "exception"},
	"fault":      {"error", "failure"},
	"slow":       {"latency", "timeout", "delay"},
	"timeout":    {"slow", "latency", "delay"},
	"memory":     {"ram", "heap", "oom"},
	"oom":        {"memory", "heap"},
	"cpu":        {"processor", "compute", "utilisation", "utilization"},
	"pod":        {"container", "workload"},
	"container":  {"pod", "workload"},
	"throughput": {"rps", "qps"},
	"rps":        {"throughput", "qps"},
	"crash":      {"restart", "oomkill", "failure"},
	"restart":    {"crash", "oomkill"},
	"saturation": {"usage", "utilisation", "utilization", "capacity"},
	"alert":      {"alarm", "notification", "firing"},
	"spike":      {"surge", "burst", "anomaly"},
	"database":   {"db", "postgres", "postgresql", "mysql", "sql"},
	"db":         {"database", "postgres", "postgresql", "mysql"},
	"postgres":   {"database", "db", "postgresql", "sql"},
	"postgresql": {"database", "db", "postgres", "sql"},
	"log":        {"logline", "entry", "event"},
	"trace":      {"span", "distributed"},
	"span":       {"trace"},
	"endpoint":   {"path", "route", "url"},
	"path":       {"endpoint", "route", "url"},
	"namespace":  {"ns", "environment", "env"},
	"node":       {"instance", "host", "machine"},
	"instance":   {"node", "host", "machine"},
}

// sreSynonyms is the stemmed version of rawSynonyms, built at init time.
var sreSynonyms map[string][]string

func init() {
	// Sort keys so map iteration produces the same sreSynonyms regardless of
	// Go's randomised map traversal order.
	keys := make([]string, 0, len(rawSynonyms))
	for k := range rawSynonyms {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sreSynonyms = make(map[string][]string, len(rawSynonyms))
	for _, k := range keys {
		sk := stemWord(k)
		seen := make(map[string]bool)
		var stemmed []string
		for _, v := range rawSynonyms[k] {
			sv := stemWord(v)
			if !seen[sv] && sv != sk {
				seen[sv] = true
				stemmed = append(stemmed, sv)
			}
		}
		sreSynonyms[sk] = append(sreSynonyms[sk], stemmed...)
	}
}

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
	idf, df := computeIDF(docs, queryTokens)
	impliedCategory := detectCategory(queryTokens)

	// Suppress results if fewer than 1/3 of the original query tokens (before
	// synonym expansion) appear in any document. A single coincidental synonym
	// hit is noise, not a match.
	// tokenize() already stems; just remove stop words without re-stemming.
	stemmedOnly := make([]string, 0)
	for _, t := range tokenize(query) {
		if !stopWords[t] {
			stemmedOnly = append(stemmedOnly, t)
		}
	}
	if len(stemmedOnly) > 0 {
		matched := 0
		for _, t := range stemmedOnly {
			if df[t] > 0 {
				matched++
				continue
			}
			// Also count tokens whose synonym appears in the corpus.
			for _, syn := range sreSynonyms[t] {
				if df[syn] > 0 {
					matched++
					break
				}
			}
		}
		if float64(matched)/float64(len(stemmedOnly)) < minTokenCoverage {
			return nil
		}
	}

	type scored struct {
		idx   int
		score float64
	}
	var scored_ []scored
	for i, tokens := range docs {
		s := bm25Score(tokens, queryTokens, idf, avgLen)
		if s > 0 {
			if impliedCategory != "" && sections[i].Category == impliedCategory {
				s *= categoryBoost
			}
			scored_ = append(scored_, scored{i, s})
		}
	}

	sort.SliceStable(scored_, func(a, b int) bool {
		if scored_[a].score != scored_[b].score {
			return scored_[a].score > scored_[b].score
		}
		// Tiebreak by section title for deterministic output.
		return sections[scored_[a].idx].Title < sections[scored_[b].idx].Title
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

// stemWord reduces a word to its Porter stem.
func stemWord(word string) string {
	stemmed, err := snowball.Stem(word, "english", true)
	if err != nil {
		return word
	}
	return stemmed
}

// expandSynonyms adds domain synonym tokens for each input token.
// The original tokens are kept; synonyms are appended and also stemmed.
func expandSynonyms(tokens []string) []string {
	seen := make(map[string]bool, len(tokens))
	out := make([]string, 0, len(tokens)*2)
	for _, t := range tokens {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
		for _, syn := range sreSynonyms[t] {
			stemmed := stemWord(syn)
			if !seen[stemmed] {
				seen[stemmed] = true
				out = append(out, stemmed)
			}
		}
	}
	return out
}

func tokenizeQuery(s string) []string {
	tokens := tokenize(s)
	// Remove stop words, then stem, then expand with synonyms.
	filtered := tokens[:0]
	for _, t := range tokens {
		if !stopWords[t] {
			filtered = append(filtered, stemWord(t))
		}
	}
	return expandSynonyms(filtered)
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) > 1 {
			// Stem every token so documents and queries share the same form.
			out = append(out, stemWord(f))
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

func computeIDF(docs [][]string, queryTokens []string) (idf map[string]float64, df map[string]int) {
	N := float64(len(docs))
	df = make(map[string]int)
	for _, tokens := range docs {
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
	}
	idf = make(map[string]float64, len(queryTokens))
	for _, t := range queryTokens {
		idf[t] = math.Log((N-float64(df[t])+0.5)/(float64(df[t])+0.5) + 1)
	}
	return idf, df
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
