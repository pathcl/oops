package adoclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/pathcl/oops/internal/config"
)

// azureDevOpsResourceID is the well-known resource ID for Azure DevOps.
const azureDevOpsResourceID = "499b84ac-1321-427f-aa17-267ca6975798"

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

// FetchMarkdown retrieves the raw content of the configured markdown file from ADO.
func (c *Client) FetchMarkdown() (string, error) {
	token, err := getAzToken()
	if err != nil {
		return "", fmt.Errorf("getting azure token: %w", err)
	}

	rawURL := c.buildURL()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching from azure devops: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("azure devops returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	return string(body), nil
}

func (c *Client) buildURL() string {
	ado := c.cfg.AzureDevOps
	base := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/git/repositories/%s/items",
		url.PathEscape(ado.Org),
		url.PathEscape(ado.Project),
		url.PathEscape(ado.Repo),
	)
	params := url.Values{}
	params.Set("path", ado.FilePath)
	params.Set("versionDescriptor.version", ado.Branch)
	params.Set("versionDescriptor.versionType", "branch")
	params.Set("$format", "text")
	params.Set("api-version", "7.1")
	return base + "?" + params.Encode()
}

type azTokenResponse struct {
	AccessToken string `json:"accessToken"`
}

func getAzToken() (string, error) {
	out, err := exec.Command("az", "account", "get-access-token",
		"--resource", azureDevOpsResourceID,
		"--output", "json",
	).Output()
	if err != nil {
		return "", fmt.Errorf("az account get-access-token failed (is az CLI installed and logged in?): %w", err)
	}

	var resp azTokenResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("parsing az token response: %w", err)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("empty access token from az CLI")
	}
	return resp.AccessToken, nil
}
