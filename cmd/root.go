package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pathcl/oops/internal/adoclient"
	"github.com/pathcl/oops/internal/cache"
	"github.com/pathcl/oops/internal/config"
	"github.com/pathcl/oops/internal/parser"
	"github.com/pathcl/oops/internal/search"
)

const maxResults = 5

var (
	refreshFlag  bool
	localFile    string
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
			printResults(search.Search(sections, query, maxResults), query)
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
		client := adoclient.New(cfg)

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

		results := search.Search(sections, query, maxResults)
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

func Execute() {
	rootCmd.Flags().BoolVarP(&refreshFlag, "refresh", "r", false, "force re-fetch from Azure DevOps (ignore cache)")
	rootCmd.Flags().StringVarP(&localFile, "file", "f", "", "use a local markdown file instead of Azure DevOps")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
