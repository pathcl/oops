package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pathcl/oops/internal/adoclient"
	"github.com/pathcl/oops/internal/cache"
	"github.com/pathcl/oops/internal/config"
	"github.com/pathcl/oops/internal/githubclient"
	"github.com/pathcl/oops/internal/llm"
	"github.com/pathcl/oops/internal/parser"
	"github.com/pathcl/oops/internal/search"
)

const (
	maxResults    = 5
	llmCandidates = 10 // wider BM25 net when LLM reranking is active
)

var (
	refreshFlag bool
	localFile   string
	useLLM      bool
)

var rootCmd = &cobra.Command{
	Use:   "oops [query]",
	Short: "SRE query cheatsheet — search PromQL, LogQL, TraceQL snippets",
	Long: `oops searches your team's cheatsheet markdown (stored in Azure DevOps)
and returns the most relevant query snippets for your question.

Example:
  oops "how do I calculate error rate?"
  oops "tail latency p99"
  oops --refresh "log filter by namespace"
  oops --file ./cheatsheet.md "error rate"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		// --file bypasses ADO, cache, and config entirely.
		if localFile != "" {
			data, err := os.ReadFile(localFile)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			sections := parser.Parse(string(data))
			if len(sections) == 0 {
				return fmt.Errorf("no sections found in %s — check the markdown format (expected ## Category > ### Title > code block)", localFile)
			}
			n := maxResults
			if useLLM {
				n = llmCandidates
			}
			results := search.Search(sections, query, n)
			if useLLM {
				results, err = rerank(cmd.Context(), query, results, config.LLM{})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: LLM rerank failed (%v) — using BM25 order\n", err)
					results = results[:min(maxResults, len(results))]
				}
			}
			printResults(results, query)
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		c := cache.New(cfg.CacheTTL)
		client := newFetcher(cfg)

		fetchFn := func() (string, error) {
			fmt.Fprintln(os.Stderr, "Fetching cheatsheet from Azure DevOps...")
			content, err := client.FetchMarkdown()
			if err != nil {
				return "", err
			}
			if err := c.Write(content); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not write cache: %v\n", err)
			}
			return content, nil
		}

		var markdown string
		if refreshFlag || c.IsStale() {
			markdown, err = fetchFn()
		} else {
			markdown, err = c.Read()
		}
		if err != nil {
			return fmt.Errorf("loading cheatsheet: %w", err)
		}

		sections := parser.Parse(markdown)
		if len(sections) == 0 {
			return fmt.Errorf("no sections found in cheatsheet — check the markdown format (expected ## Category > ### Title > code block)")
		}

		n := maxResults
		if useLLM {
			n = llmCandidates
		}
		results := search.Search(sections, query, n)
		if useLLM {
			results, err = rerank(cmd.Context(), query, results, cfg.LLM)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: LLM rerank failed (%v) — using BM25 order\n", err)
				results = results[:min(maxResults, len(results))]
			}
		}
		printResults(results, query)
		return nil
	},
}

func printResults(results []search.Result, query string) {
	if len(results) == 0 {
		fmt.Printf("No results for %q\n", query)
		return
	}
	fmt.Printf("%d result(s) for %q\n", len(results), query)
	for _, r := range results {
		s := r.Section
		fmt.Printf("\n[%s] %s\n", s.Category, s.Title)
		if s.Body != "" {
			fmt.Printf("%s\n", s.Body)
		}
		if s.CodeBlock != "" {
			fmt.Println()
			for _, line := range strings.Split(s.CodeBlock, "\n") {
				fmt.Printf("    %s\n", line)
			}
		}
	}
	fmt.Println()
}

// Fetcher is the common interface for all remote cheatsheet sources.
type Fetcher interface {
	FetchMarkdown() (string, error)
}

// newFetcher picks the right client based on which config block is populated.
func newFetcher(cfg *config.Config) Fetcher {
	if cfg.GitHub.Owner != "" {
		return githubclient.New(cfg)
	}
	return adoclient.New(cfg)
}

// rerank runs the LLM reranker, prints token usage to stderr, and trims to maxResults.
func rerank(ctx context.Context, query string, results []search.Result, cfg config.LLM) ([]search.Result, error) {
	r, err := llm.New(llm.Config{
		Provider: cfg.Provider,
		Model:    cfg.Model,
		APIKey:   cfg.APIKey,
	})
	if err != nil {
		return nil, err
	}
	reranked, usage, err := r.Rerank(ctx, query, results)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "LLM rerank: %s\n", usage)
	if len(reranked) > maxResults {
		reranked = reranked[:maxResults]
	}
	return reranked, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Execute() {
	rootCmd.Flags().BoolVarP(&refreshFlag, "refresh", "r", false, "force re-fetch from Azure DevOps (ignore cache)")
	rootCmd.Flags().StringVarP(&localFile, "file", "f", "", "use a local markdown file instead of Azure DevOps")
	rootCmd.Flags().BoolVar(&useLLM, "llm", false, "rerank BM25 results using an LLM (requires ANTHROPIC_API_KEY or OPENAI_API_KEY)")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
