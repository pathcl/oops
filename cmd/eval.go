package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pathcl/oops/internal/config"
	"github.com/pathcl/oops/internal/eval"
	"github.com/pathcl/oops/internal/llm"
	"github.com/pathcl/oops/internal/parser"
)

var evalCmd = &cobra.Command{
	Use:   "eval --file <cheatsheet.md> [--queries <eval_queries.yaml>] [--llm]",
	Short: "Evaluate BM25 vs BM25+LLM using ground-truth queries",
	Long: `Runs a set of ground-truth queries against the cheatsheet and reports
Precision@1 and MRR for BM25 alone and BM25+LLM side by side.

Example:
  oops eval --file testdata/cheatsheet.md --queries testdata/eval_queries.yaml
  oops eval --file testdata/cheatsheet.md --queries testdata/eval_queries.yaml --llm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		evalFile, _ := cmd.Flags().GetString("queries")
		markdownFile, _ := cmd.Flags().GetString("file")
		withLLM, _ := cmd.Flags().GetBool("llm")

		if markdownFile == "" {
			return fmt.Errorf("--file is required")
		}
		if evalFile == "" {
			return fmt.Errorf("--queries is required")
		}

		data, err := os.ReadFile(markdownFile)
		if err != nil {
			return fmt.Errorf("reading cheatsheet: %w", err)
		}
		sections := parser.Parse(string(data))
		if len(sections) == 0 {
			return fmt.Errorf("no sections found in %s", markdownFile)
		}

		cases, err := eval.LoadQueries(evalFile)
		if err != nil {
			return err
		}

		var reranker llm.Reranker
		if withLLM {
			cfg, _ := config.Load()
			var llmCfg config.LLM
			if cfg != nil {
				llmCfg = cfg.LLM
			}
			reranker, err = llm.New(llm.Config{
				Provider: llmCfg.Provider,
				Model:    llmCfg.Model,
				APIKey:   llmCfg.APIKey,
			})
			if err != nil {
				return fmt.Errorf("creating reranker: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Running evaluation with LLM reranking...\n")
		} else {
			fmt.Fprintf(os.Stderr, "Running BM25-only evaluation (pass --llm to compare with LLM)...\n")
		}

		report := eval.Run(context.Background(), cases, sections, reranker)
		eval.Print(os.Stdout, report)
		return nil
	},
}

func init() {
	evalCmd.Flags().String("file", "", "path to cheatsheet markdown")
	evalCmd.Flags().String("queries", "", "path to ground-truth YAML (default: testdata/eval_queries.yaml)")
	evalCmd.Flags().Bool("llm", false, "also run BM25+LLM and compare")
	rootCmd.AddCommand(evalCmd)
}
