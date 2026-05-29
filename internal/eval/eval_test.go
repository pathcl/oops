package eval

import (
	"testing"

	"github.com/pathcl/oops/internal/parser"
	"github.com/pathcl/oops/internal/search"
)

func TestRecipRank(t *testing.T) {
	results := []search.Result{
		{Section: parser.Section{Title: "A"}},
		{Section: parser.Section{Title: "B"}},
		{Section: parser.Section{Title: "C"}},
	}
	if got := recipRank(results, "A"); got != 1.0 {
		t.Errorf("recipRank top = %f, want 1.0", got)
	}
	if got := recipRank(results, "B"); got != 0.5 {
		t.Errorf("recipRank second = %f, want 0.5", got)
	}
	if got := recipRank(results, "C"); got != 1.0/3 {
		t.Errorf("recipRank third = %f, want 0.333", got)
	}
	if got := recipRank(results, "Z"); got != 0 {
		t.Errorf("recipRank miss = %f, want 0", got)
	}
}

func TestPrecisionAt1(t *testing.T) {
	results := []search.Result{
		{Section: parser.Section{Title: "Right"}},
		{Section: parser.Section{Title: "Wrong"}},
	}
	if got := precisionAt1(results, "Right"); got != 1.0 {
		t.Errorf("P@1 hit = %f, want 1.0", got)
	}
	if got := precisionAt1(results, "Wrong"); got != 0.0 {
		t.Errorf("P@1 miss = %f, want 0.0", got)
	}
}

func TestMRR(t *testing.T) {
	cases := []QueryResult{
		{RR: 1.0},
		{RR: 0.5},
		{RR: 0.0},
	}
	got := mrr(cases)
	want := (1.0 + 0.5 + 0.0) / 3
	if got != want {
		t.Errorf("MRR = %f, want %f", got, want)
	}
}

func TestSummary_EasyVsHard(t *testing.T) {
	results := []QueryResult{
		{Difficulty: "easy", P1: 1.0, RR: 1.0},
		{Difficulty: "easy", P1: 1.0, RR: 1.0},
		{Difficulty: "hard", P1: 0.0, RR: 0.5},
		{Difficulty: "hard", P1: 1.0, RR: 1.0},
	}
	easy, hard := splitByDifficulty(results)
	if len(easy) != 2 || len(hard) != 2 {
		t.Errorf("split: easy=%d hard=%d, want 2/2", len(easy), len(hard))
	}
	if mrr(easy) != 1.0 {
		t.Errorf("easy MRR = %f, want 1.0", mrr(easy))
	}
}
