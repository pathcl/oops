package githubclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/pathcl/oops/internal/config"
)

// Client fetches a markdown file from a GitHub repository using the gh CLI for auth.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchMarkdown retrieves the raw content of the configured markdown file from GitHub.
func (c *Client) FetchMarkdown() (string, error) {
	token, err := getGHToken()
	if err != nil {
		return "", fmt.Errorf("getting github token: %w", err)
	}

	rawURL := c.buildURL()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.raw")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching from github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	return string(body), nil
}

func (c *Client) buildURL() string {
	gh := c.cfg.GitHub
	return fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(gh.Owner),
		url.PathEscape(gh.Repo),
		strings.TrimPrefix(gh.FilePath, "/"),
		url.QueryEscape(gh.Branch),
	)
}

func getGHToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed (is gh CLI installed and logged in?): %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token from gh CLI")
	}
	return token, nil
}
