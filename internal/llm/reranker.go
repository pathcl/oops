package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/pathcl/oops/internal/search"
)

// Reranker reorders BM25 candidates by semantic relevance to the query.
type Reranker interface {
	Rerank(ctx context.Context, query string, results []search.Result) ([]search.Result, error)
}

// Config holds the LLM provider settings.
type Config struct {
	Provider string // "anthropic" or "openai"
	Model    string // e.g. "claude-haiku-4-5" or "gpt-4o-mini"
	APIKey   string // falls back to ANTHROPIC_API_KEY / OPENAI_API_KEY env vars
}

// New returns a Reranker for the configured provider.
func New(cfg Config) (Reranker, error) {
	switch strings.ToLower(cfg.Provider) {
	case "anthropic", "":
		model := cfg.Model
		if model == "" {
			model = string(anthropic.ModelClaudeHaiku4_5_20251001)
		}
		opts := []option.RequestOption{}
		if cfg.APIKey != "" {
			opts = append(opts, option.WithAPIKey(cfg.APIKey))
		}
		return &anthropicReranker{
			client: anthropic.NewClient(opts...),
			model:  model,
		}, nil
	case "openai":
		model := cfg.Model
		if model == "" {
			model = "gpt-4o-mini"
		}
		return &openAIReranker{
			model:  model,
			apiKey: cfg.APIKey,
			http:   &http.Client{Timeout: 30 * time.Second},
		}, nil
	default:
		return nil, fmt.Errorf("unknown llm provider %q (supported: anthropic, openai)", cfg.Provider)
	}
}

// --- Anthropic implementation ---

type anthropicReranker struct {
	client anthropic.Client
	model  string
}

func (r *anthropicReranker) Rerank(ctx context.Context, query string, results []search.Result) ([]search.Result, error) {
	if len(results) == 0 {
		return results, nil
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ranking": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer"},
				"description": "0-based indices of the input snippets ordered from most to least relevant",
			},
		},
		"required":             []string{"ranking"},
		"additionalProperties": false,
	}

	resp, err := r.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(r.model),
		MaxTokens: 512,
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: schema,
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(buildPrompt(query, results))),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic rerank: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text = t.Text
			break
		}
	}

	ranking, err := parseRanking(text)
	if err != nil {
		return results, nil // fall back to BM25 order on parse failure
	}
	return applyRanking(results, ranking), nil
}

// --- OpenAI implementation (raw HTTP) ---

type openAIReranker struct {
	model  string
	apiKey string
	http   *http.Client
}

type openAIChatRequest struct {
	Model          string             `json:"model"`
	Messages       []openAIMessage    `json:"messages"`
	ResponseFormat openAIRespFormat   `json:"response_format"`
	MaxTokens      int                `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRespFormat struct {
	Type string `json:"type"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (r *openAIReranker) Rerank(ctx context.Context, query string, results []search.Result) ([]search.Result, error) {
	if len(results) == 0 {
		return results, nil
	}

	body, err := json.Marshal(openAIChatRequest{
		Model: r.model,
		Messages: []openAIMessage{
			{Role: "user", Content: buildPrompt(query, results)},
		},
		ResponseFormat: openAIRespFormat{Type: "json_object"},
		MaxTokens:      512,
	})
	if err != nil {
		return nil, fmt.Errorf("openai marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("openai decode: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return results, nil
	}

	ranking, err := parseRanking(chatResp.Choices[0].Message.Content)
	if err != nil {
		return results, nil
	}
	return applyRanking(results, ranking), nil
}

// --- shared helpers ---

func buildPrompt(query string, results []search.Result) string {
	var b strings.Builder
	b.WriteString("You are an SRE query search assistant. Rerank the following snippets by relevance to the query.\n\n")
	b.WriteString("Query: ")
	b.WriteString(query)
	b.WriteString("\n\nSnippets:\n")
	for i, r := range results {
		s := r.Section
		fmt.Fprintf(&b, "%d. [%s] %s\n   %s\n   %s\n\n", i, s.Category, s.Title, s.Body, s.CodeBlock)
	}
	b.WriteString(`Return JSON: {"ranking": [<indices most to least relevant>]}`)
	return b.String()
}

type rankingResponse struct {
	Ranking []int `json:"ranking"`
}

func parseRanking(text string) ([]int, error) {
	// extract first JSON object in case the model wraps it in prose
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON object in response")
	}
	var r rankingResponse
	if err := json.Unmarshal([]byte(text[start:end+1]), &r); err != nil {
		return nil, fmt.Errorf("parse ranking JSON: %w", err)
	}
	if len(r.Ranking) == 0 {
		return nil, fmt.Errorf("empty ranking array")
	}
	return r.Ranking, nil
}

func applyRanking(results []search.Result, ranking []int) []search.Result {
	out := make([]search.Result, 0, len(ranking))
	for _, idx := range ranking {
		if idx >= 0 && idx < len(results) {
			out = append(out, results[idx])
		}
	}
	return out
}
