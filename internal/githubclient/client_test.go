package githubclient

import (
	"testing"

	"github.com/pathcl/oops/internal/config"
)

func TestBuildURL(t *testing.T) {
	cases := []struct {
		cfg  config.GitHub
		want string
	}{
		{
			cfg:  config.GitHub{Owner: "my-org", Repo: "sre-runbooks", FilePath: "docs/cheatsheet.md", Branch: "main"},
			want: "https://api.github.com/repos/my-org/sre-runbooks/contents/docs/cheatsheet.md?ref=main",
		},
		{
			// leading slash in FilePath should be stripped
			cfg:  config.GitHub{Owner: "acme", Repo: "ops", FilePath: "/runbooks/oops.md", Branch: "develop"},
			want: "https://api.github.com/repos/acme/ops/contents/runbooks/oops.md?ref=develop",
		},
		{
			// org/repo with special characters should be escaped
			cfg:  config.GitHub{Owner: "my org", Repo: "my repo", FilePath: "file.md", Branch: "main"},
			want: "https://api.github.com/repos/my%20org/my%20repo/contents/file.md?ref=main",
		},
	}

	for _, c := range cases {
		client := &Client{cfg: &config.Config{GitHub: c.cfg}}
		got := client.buildURL()
		if got != c.want {
			t.Errorf("buildURL() = %q, want %q", got, c.want)
		}
	}
}

func TestGetGHToken_ErrorWhenNotInstalled(t *testing.T) {
	// We can't easily mock exec.Command, but we can verify the function
	// returns a non-empty error message when gh is not available or not logged in.
	// This test just validates the error path is reachable; it will skip
	// if gh happens to be installed and logged in.
	_, err := getGHToken()
	if err == nil {
		t.Log("gh CLI is installed and logged in — token fetch succeeded (expected in CI with gh)")
	} else {
		// Error should mention gh CLI
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	}
}
